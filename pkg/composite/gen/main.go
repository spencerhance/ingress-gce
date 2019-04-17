/*
Copyright 2018 Google LLC
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
https://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Generator for GCE compute wrapper code. You must regenerate the code after
// modifying this file:
//
//   $ ./hack/update_codegen.sh

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/util/sets"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/template"
	"time"
	"unicode"
)

const (
	gofmt = "gofmt"

	// This assumes that alpha contains a superset of all struct fields
	apiFilePath = "./vendor/google.golang.org/api/compute/v0.alpha/compute-api.json"
)

// MainServices describes all of the API types that we want to define all the helper functions for
// The other types that are discovered as dependencies will simply be wrapped with a composite struct
// The format of the map is ServiceName -> k8s-cloud-provider wrapper name
var MainServices = map[string]string{
	"BackendService":   "BackendServices",
	"ForwardingRule":   "ForwardingRules",
	"HttpHealthCheck":  "HttpHealthChecks",
	"HttpsHealthCheck": "HttpsHealthChecks",
	"UrlMap":           "UrlMaps",
	"TargetHttpProxy":  "TargetHttpProxies",
	"TargetHttpsProxy": "TargetHttpsProxies",
}

// TODO: (shance) Replace this with data gathered from meta.AllServices
// Services in NoUpdate will not have an Update() method generated for them
var NoUpdate = sets.NewString(
	"ForwardingRule",
	"TargetHttpProxy",
	"TargetHttpsProxy",
)

var Versions = map[string]string{
	"Alpha": "alpha",
	"Beta":  "beta",
	"GA":    "",
}

// ApiService is a struct to hold all of the relevant data for generating a composite service
type ApiService struct {
	Name     string
	JsonName string
	// Force JSON tag as string type
	JsonStringOverride bool
	// Golang type
	GoType  string
	VarName string
	Fields  []ApiService
}

func (apiService *ApiService) IsMainService() bool {
	_, found := MainServices[apiService.Name]
	return found
}

func (apiService *ApiService) HasUpdate() bool {
	return !NoUpdate.Has(apiService.Name)
}

func (apiService *ApiService) GetCloudProviderName() string {
	result, ok := MainServices[apiService.Name]
	if !ok {
		panic(fmt.Errorf("%s not present in map: %v", apiService.Name, MainServices))
	}

	return result
}

var AllApiServices []ApiService

// gofmtContent runs "gofmt" on the given contents.
// Duplicate of the function in k8s-cloud-provider
func gofmtContent(r io.Reader) string {
	cmd := exec.Command(gofmt, "-s")
	out := &bytes.Buffer{}
	cmd.Stdin = r
	cmd.Stdout = out
	cmdErr := &bytes.Buffer{}
	cmd.Stderr = cmdErr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, cmdErr.String())
		panic(err)
	}
	return out.String()
}

// createVarName() converts the service name into camelcase
func createVarName(str string) string {
	copy := []rune(str)
	if len(copy) == 0 {
		return string(copy)
	}

	copy[0] = unicode.ToLower(rune(copy[0]))
	return string(copy)
}

// populateApiServices() parses the Api Spec and populates AllApiServices with the required services
// Performs BFS to resolve dependencies
func populateApiServices() {
	apiFile, err := os.Open(apiFilePath)
	if err != nil {
		panic(err)
	}
	defer apiFile.Close()

	byteValue, err := ioutil.ReadAll(apiFile)
	if err != nil {
		panic(err)
	}

	var result map[string]interface{}
	json.Unmarshal([]byte(byteValue), &result)

	// Queue of ApiService names for BFS
	typesQueue := []string{}

	// Set of already parsed ApiService names for BFS
	completed := sets.String{}

	// Go type of the property
	var propType string

	keys := []string{}
	for key := range MainServices {
		keys = append(keys, key)
	}
	typesQueue = append(typesQueue, keys...)

	for len(typesQueue) > 0 {
		typeName := typesQueue[0]
		typesQueue = typesQueue[1:]

		if completed.Has(typeName) {
			continue
		}
		completed.Insert(typeName)

		fields, ok := result["schemas"].
			(map[string]interface{})[typeName].
			(map[string]interface{})["properties"].
			(map[string]interface{})
		if !ok {
			panic(fmt.Errorf("Unable to parse type: %s", typeName))
		}

		apiService := ApiService{Name: typeName, Fields: []ApiService{}, VarName: createVarName(typeName)}

		for prop, val := range fields {
			subType := ApiService{Name: strings.Title(prop), JsonName: prop}

			var override bool
			propType, typesQueue, override, err = getGoType(val, typesQueue)
			if err != nil {
				panic(err)
			}
			subType.GoType = propType
			subType.JsonStringOverride = override
			apiService.Fields = append(apiService.Fields, subType)
		}

		// Sort fields since the keys aren't ordered deterministically
		sort.Slice(apiService.Fields[:], func(i, j int) bool {
			return apiService.Fields[i].Name < apiService.Fields[j].Name
		})

		AllApiServices = append(AllApiServices, apiService)
	}

	// Sort the struct definitions since the keys aren't ordered deterministically
	sort.Slice(AllApiServices[:], func(i, j int) bool {
		return AllApiServices[i].Name < AllApiServices[j].Name
	})
}

// getGoType() determines what the golang type is for a service by recursively descending the API spec json
// for a field.  Since this may discover new types, it also updates the typesQueue.
func getGoType(val interface{}, typesQueue []string) (string, []string, bool, error) {
	field, ok := val.(map[string]interface{})
	if !ok {
		panic(nil)
	}

	var err error
	var tmpType string
	var override bool

	propType := ""
	ref, ok := field["$ref"]
	// Field is not a built-in type, we need to wrap it
	if ok {
		refName := ref.(string)
		typesQueue = append(typesQueue, refName)
		propType = "*" + refName
	} else if field["type"] == "array" {
		tmpType, typesQueue, override, err = getGoType(field["items"], typesQueue)
		propType = "[]" + tmpType
	} else if field["type"] == "object" {
		addlProps, ok := field["additionalProperties"]
		if ok {
			tmpType, typesQueue, override, err = getGoType(addlProps, typesQueue)
			propType = "map[string]" + tmpType
		} else {
			propType = "map[string]string"
		}
	} else if format, ok := field["format"]; ok {
		if format.(string) == "byte" {
			propType = "string"
		} else if format.(string) == "float" {
			propType = "float64"
		} else if format.(string) == "int32" {
			propType = "int64"
		} else {
			propType = format.(string)
		}
	} else if field["type"] != "" {
		if field["type"].(string) == "boolean" {
			propType = "bool"
		} else {
			propType = field["type"].(string)
		}
	} else {
		err = fmt.Errorf("unable to get property type for prop: %v", val)
	}

	if field["type"] == "string" && propType != "string" {
		override = true
	}

	return propType, typesQueue, override, err
}

func genHeader(wr io.Writer) {
	const text = `/*
Copyright {{.Year}} Google LLC
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
https://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// This file was generated by "go run gen/main.go". Do not edit directly.
// directly.

package composite
import (
	"encoding/json"
	"fmt"

	"k8s.io/klog"

	computealpha "google.golang.org/api/compute/v0.alpha"
	computebeta "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	gcecloud "github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
)
`
	tmpl := template.Must(template.New("header").Parse(text))
	values := map[string]string{
		"Year": fmt.Sprintf("%v", time.Now().Year()),
	}
	if err := tmpl.Execute(wr, values); err != nil {
		panic(err)
	}

	fmt.Fprintf(wr, "\n\n")
}

func genTestHeader(wr io.Writer) {
	const text = `/*
Copyright {{.Year}} Google LLC
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
https://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// This file was generated by "go run gen/main.go". Do not edit directly.
// directly.

package composite
import (
	"fmt"
	"reflect"
	"testing"

	"github.com/kr/pretty"
	computealpha "google.golang.org/api/compute/v0.alpha"
	computebeta "google.golang.org/api/compute/v0.beta"
	compute "google.golang.org/api/compute/v1"
)
`
	tmpl := template.Must(template.New("testHeader").Parse(text))
	values := map[string]string{
		"Year": fmt.Sprintf("%v", time.Now().Year()),
	}
	if err := tmpl.Execute(wr, values); err != nil {
		panic(err)
	}

	fmt.Fprintf(wr, "\n\n")
}

// genTypes() generates all of the struct definitions
func genTypes(wr io.Writer) {
	const text = `
{{ $backtick := "` + "`" + `" }}
{{- range .All}}
	// {{.Name}} is a composite type wrapping the Alpha, Beta, and GA methods for its GCE equivalent
	type {{.Name}} struct {
		{{- if .IsMainService}}
			// Version keeps track of the intended compute version for this {{.Name}}.
			// Note that the compute API's do not contain this field. It is for our
			// own bookkeeping purposes.
			Version meta.Version
		{{- end}}

		{{- range .Fields}}
			{{- if eq .Name "Id"}}
				{{.Name}} {{.GoType}} {{$backtick}}json:"{{.JsonName}},omitempty,string"{{$backtick}}
			{{- else if .JsonStringOverride}}
				{{.Name}} {{.GoType}} {{$backtick}}json:"{{.JsonName}},omitempty,string"{{$backtick}}
			{{- else}}
				{{.Name}} {{.GoType}} {{$backtick}}json:"{{.JsonName}},omitempty"{{$backtick}}
			{{- end}}
		{{- end}}
		{{- if .IsMainService}}
			googleapi.ServerResponse {{$backtick}}json:"-"{{$backtick}}
		{{- end}}
		ForceSendFields []string {{$backtick}}json:"-"{{$backtick}}
		NullFields []string {{$backtick}}json:"-"{{$backtick}}
}
{{- end}}
`
	data := struct {
		All []ApiService
	}{AllApiServices}

	tmpl := template.Must(template.New("types").Parse(text))
	if err := tmpl.Execute(wr, data); err != nil {
		panic(err)
	}
}

// genFuncs() generates all of the struct methods
// TODO: (shance) generated CRUD functions should take a meta.Key object to allow easier use of global and regional resources
func genFuncs(wr io.Writer) {
	const text = `
{{$All := .All}}
{{$Versions := .Versions}}

{{range $type := $All}}
{{if .IsMainService}}
	func Create{{.Name}}({{.VarName}} *{{.Name}}, cloud *gce.Cloud, key *meta.Key) error {
	ctx, cancel := gcecloud.ContextWithCallTimeout()
	defer cancel()

	switch {{.VarName}}.Version {
	case meta.VersionAlpha:
		alpha, err := {{.VarName}}.toAlpha()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Creating alpha {{.Name}} %v", alpha.Name)
		return cloud.Compute().Alpha{{.GetCloudProviderName}}().Insert(ctx, key, alpha)
	case meta.VersionBeta:
		beta, err := {{.VarName}}.toBeta()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Creating beta {{.Name}} %v", beta.Name)
		return cloud.Compute().Beta{{.GetCloudProviderName}}().Insert(ctx, key, beta)
	default:
		ga, err := {{.VarName}}.toGA()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Creating ga {{.Name}} %v", ga.Name)
		return cloud.Compute().{{.GetCloudProviderName}}().Insert(ctx, key, ga)
	}
}

{{if .HasUpdate}}
func Update{{.Name}}({{.VarName}} *{{.Name}}, cloud *gce.Cloud, key *meta.Key) error {
	ctx, cancel := gcecloud.ContextWithCallTimeout()
	defer cancel()	

	switch {{.VarName}}.Version {
	case meta.VersionAlpha:
		alpha, err := {{.VarName}}.toAlpha()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Updating alpha {{.Name}} %v", alpha.Name)
		return cloud.Compute().Alpha{{.GetCloudProviderName}}().Update(ctx, key, alpha)
	case meta.VersionBeta:
		beta, err := {{.VarName}}.toBeta()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Updating beta {{.Name}} %v", beta.Name)
		return cloud.Compute().Beta{{.GetCloudProviderName}}().Update(ctx, key, beta)
	default:
		ga, err := {{.VarName}}.toGA()
		if err != nil {
			return err
		}
		klog.V(3).Infof("Updating ga {{.Name}} %v", ga.Name)
		return cloud.Compute().{{.GetCloudProviderName}}().Update(ctx, key, ga)
	}
}
{{- end}}

func Get{{.Name}}(name string, version meta.Version, cloud *gce.Cloud, key *meta.Key) (*{{.Name}}, error) {
	ctx, cancel := gcecloud.ContextWithCallTimeout()
	defer cancel()	

	var gceObj interface{}
	var err error
	switch version {
	case meta.VersionAlpha:
		gceObj, err = cloud.Compute().Alpha{{.GetCloudProviderName}}().Get(ctx, key)
	case meta.VersionBeta:
		gceObj, err = cloud.Compute().Beta{{.GetCloudProviderName}}().Get(ctx, key)
	default:
		gceObj, err = cloud.Compute().{{.GetCloudProviderName}}().Get(ctx, key)
	}
	if err != nil {
		return nil, err
	}
	return to{{.Name}}(gceObj)
}

// to{{.Name}} converts a compute alpha, beta or GA
// {{.Name}} into our composite type.
func to{{.Name}}(obj interface{}) (*{{.Name}}, error) {
	be := &{{.Name}}{}
	bytes, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("could not marshal object %+v to JSON: %v", obj, err)
	}
	err = json.Unmarshal(bytes, be)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling to {{.Name}}: %v", err)
	}
	return be, nil
}

{{- range $version, $extension := $.Versions}}
{{$lower := $version | ToLower}}
// to{{$version}} converts our composite type into an alpha type.
// This alpha type can be used in GCE API calls.
func ({{$type.VarName}} *{{$type.Name}}) to{{$version}}() (*compute{{$extension}}.{{$type.Name}}, error) {
	bytes, err := json.Marshal({{$type.VarName}})
	if err != nil {
		return nil, fmt.Errorf("error marshalling {{$type.Name}} to JSON: %v", err)
	}
	{{$version | ToLower}} := &compute{{$extension}}.{{$type.Name}}{}
	err = json.Unmarshal(bytes, {{$lower}})
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling {{$type.Name}} JSON to compute {{$lower}} type: %v", err)
	}

	{{- if eq $type.Name "BackendService"}}
	// Set force send fields. This is a temporary hack.
	if {{$lower}}.CdnPolicy != nil && {{$lower}}.CdnPolicy.CacheKeyPolicy != nil {
		{{$lower}}.CdnPolicy.CacheKeyPolicy.ForceSendFields = []string{"IncludeHost", "IncludeProtocol", "IncludeQueryString", "QueryStringBlacklist", "QueryStringWhitelist"}
	}
	if {{$lower}}.Iap != nil {
		{{$lower}}.Iap.ForceSendFields = []string{"Enabled", "Oauth2ClientId", "Oauth2ClientSecret"}
	}
	{{- end}}	

	return {{$lower}}, nil
}
{{- end}}


{{- end}}
{{- end}}
`
	data := struct {
		All      []ApiService
		Versions map[string]string
	}{AllApiServices, Versions}

	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
	}

	tmpl := template.Must(template.New("funcs").Funcs(funcMap).Parse(text))
	if err := tmpl.Execute(wr, data); err != nil {
		panic(err)
	}
}

// genTests() generates all of the tests
// TODO: (shance) figure out a better way to test the toGA(), toAlpha(), and toBeta() functions
func genTests(wr io.Writer) {
	const text = `
// compareFields verifies that two fields in a struct have the same relevant metadata.
// Note: This comparison ignores field offset, index, and pkg path, all of which don't matter.
func compareFields(s1, s2 reflect.StructField) error {
	if s1.Name != s2.Name {
		return fmt.Errorf("field %s name = %q, want %q", s1.Name, s1.Name, s2.Name)
	}
	if s1.Tag != s2.Tag {
		return fmt.Errorf("field %s tag = %q, want %q", s1.Name, s1.Tag, s2.Tag)
	}
	if s1.Type.Name() != s2.Type.Name() {
		return fmt.Errorf("field %s type = %q, want %q", s1.Name, s1.Type.Name(), s2.Type.Name())
	}
	return nil
}

// typeEquality is a generic function that checks type equality.
func typeEquality(t1, t2 reflect.Type) error {
	t1Fields, t2Fields := make(map[string]bool), make(map[string]bool)
	for i := 0; i < t1.NumField(); i++ {
		t1Fields[t1.Field(i).Name] = true
	}
	for i := 0; i < t2.NumField(); i++ {
		t2Fields[t2.Field(i).Name] = true
	}
	if !reflect.DeepEqual(t1Fields, t2Fields) {
		return fmt.Errorf("type = %+v, want %+v", t1Fields, t2Fields)
	}
	for n := range t1Fields {
		f1, _ := t1.FieldByName(n)
		f2, _ := t2.FieldByName(n)
		if err := compareFields(f1, f2); err != nil {
			return err
		}
	}
	return nil
}

{{- range .All}}
		{{- if .IsMainService}}
			func Test{{.Name}}(t *testing.T) {
	// Use reflection to verify that our composite type contains all the
	// same fields as the alpha type.
	compositeType := reflect.TypeOf(ForwardingRule{})
	alphaType := reflect.TypeOf(computealpha.ForwardingRule{})

	// For the composite type, remove the Version field from consideration
	compositeTypeNumFields := compositeType.NumField() - 1
	if compositeTypeNumFields != alphaType.NumField() {
		t.Fatalf("%v should contain %v fields. Got %v", alphaType.Name(), alphaType.NumField(), compositeTypeNumFields)
	}

	// Compare all the fields by doing a lookup since we can't guarantee that they'll be in the same order
	for i := 1; i < compositeType.NumField(); i++ {
		lookupField, found := alphaType.FieldByName(compositeType.Field(i).Name)
		if !found {
			t.Fatal(fmt.Errorf("Field %v not present in alpha type %v", compositeType.Field(i), alphaType))
		}
		if err := compareFields(compositeType.Field(i), lookupField); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTo{{.Name}}(t *testing.T) {
	testCases := []struct {
		input    interface{}
		expected *{{.Name}}
	}{
		{
			computealpha.{{.Name}}{},
			&{{.Name}}{},
		},
		{
			computebeta.{{.Name}}{},
			&{{.Name}}{},
		},
		{
			compute.{{.Name}}{},
			&{{.Name}}{},
		},
	}
	for _, testCase := range testCases {
		result, _ := to{{.Name}}(testCase.input)
		if !reflect.DeepEqual(result, testCase.expected) {
			t.Fatalf("to{{.Name}}(input) = \ninput = %s\n%s\nwant = \n%s", pretty.Sprint(testCase.input), pretty.Sprint(result), pretty.Sprint(testCase.expected))
		}
	}
}

{{- else}}

func Test{{.Name}}(t *testing.T) {
	compositeType := reflect.TypeOf({{.Name}}{})
	alphaType := reflect.TypeOf(computealpha.{{.Name}}{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}
		{{- end}}
{{- end}}
`
	data := struct {
		All []ApiService
	}{AllApiServices}

	tmpl := template.Must(template.New("tests").Parse(text))
	if err := tmpl.Execute(wr, data); err != nil {
		panic(err)
	}
}

func init() {
	AllApiServices = []ApiService{}
	populateApiServices()
}

func main() {
	out := &bytes.Buffer{}
	testOut := &bytes.Buffer{}

	genHeader(out)
	genTypes(out)
	genFuncs(out)

	genTestHeader(testOut)
	genTests(testOut)

	var err error
	err = ioutil.WriteFile("./pkg/composite/composite.go", []byte(gofmtContent(out)), 0644)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile("./pkg/composite/composite_test.go", []byte(gofmtContent(testOut)), 0644)
	if err != nil {
		panic(err)
	}
}
