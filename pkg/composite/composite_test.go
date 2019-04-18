/*
Copyright 2019 Google LLC
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

func TestBackend(t *testing.T) {
	compositeType := reflect.TypeOf(Backend{})
	alphaType := reflect.TypeOf(computealpha.Backend{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}
func TestBackendService(t *testing.T) {
	// Use reflection to verify that our composite type contains all the
	// same fields as the alpha type.
	compositeType := reflect.TypeOf(BackendService{})
	alphaType := reflect.TypeOf(computealpha.BackendService{})

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

func TestToBackendService(t *testing.T) {
	testCases := []struct {
		input    interface{}
		expected *BackendService
	}{
		{
			computealpha.BackendService{},
			&BackendService{},
		},
		{
			computebeta.BackendService{},
			&BackendService{},
		},
		{
			compute.BackendService{},
			&BackendService{},
		},
	}
	for _, testCase := range testCases {
		result, _ := toBackendService(testCase.input)
		if !reflect.DeepEqual(result, testCase.expected) {
			t.Fatalf("toBackendService(input) = \ninput = %s\n%s\nwant = \n%s", pretty.Sprint(testCase.input), pretty.Sprint(result), pretty.Sprint(testCase.expected))
		}
	}
}

func TestBackendServiceAppEngineBackend(t *testing.T) {
	compositeType := reflect.TypeOf(BackendServiceAppEngineBackend{})
	alphaType := reflect.TypeOf(computealpha.BackendServiceAppEngineBackend{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}

func TestBackendServiceCdnPolicy(t *testing.T) {
	compositeType := reflect.TypeOf(BackendServiceCdnPolicy{})
	alphaType := reflect.TypeOf(computealpha.BackendServiceCdnPolicy{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}

func TestBackendServiceCloudFunctionBackend(t *testing.T) {
	compositeType := reflect.TypeOf(BackendServiceCloudFunctionBackend{})
	alphaType := reflect.TypeOf(computealpha.BackendServiceCloudFunctionBackend{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}

func TestBackendServiceFailoverPolicy(t *testing.T) {
	compositeType := reflect.TypeOf(BackendServiceFailoverPolicy{})
	alphaType := reflect.TypeOf(computealpha.BackendServiceFailoverPolicy{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}

func TestBackendServiceIAP(t *testing.T) {
	compositeType := reflect.TypeOf(BackendServiceIAP{})
	alphaType := reflect.TypeOf(computealpha.BackendServiceIAP{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}

func TestBackendServiceIAPOAuth2ClientInfo(t *testing.T) {
	compositeType := reflect.TypeOf(BackendServiceIAPOAuth2ClientInfo{})
	alphaType := reflect.TypeOf(computealpha.BackendServiceIAPOAuth2ClientInfo{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}

func TestCacheKeyPolicy(t *testing.T) {
	compositeType := reflect.TypeOf(CacheKeyPolicy{})
	alphaType := reflect.TypeOf(computealpha.CacheKeyPolicy{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}

func TestConnectionDraining(t *testing.T) {
	compositeType := reflect.TypeOf(ConnectionDraining{})
	alphaType := reflect.TypeOf(computealpha.ConnectionDraining{})
	if err := typeEquality(compositeType, alphaType); err != nil {
		t.Fatal(err)
	}
}
