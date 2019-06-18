/*
Copyright 2018 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backends

import (
	lbfeatures "k8s.io/ingress-gce/pkg/loadbalancers/features"
	"net/http"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	"google.golang.org/api/compute/v1"
	"k8s.io/ingress-gce/pkg/backends/features"
	"k8s.io/ingress-gce/pkg/composite"
	"k8s.io/ingress-gce/pkg/utils"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/cloudprovider/providers/gce"
)

// Backends handles CRUD operations for backends.
type Backends struct {
	cloud          *gce.Cloud
	namer          *utils.Namer
	compositeCloud *composite.Cloud
}

// Backends is a Pool.
var _ Pool = (*Backends)(nil)

// NewPool returns a new backend pool.
// - cloud: implements BackendServices
// - namer: produces names for backends.
func NewPool(cloud *gce.Cloud, namer *utils.Namer) *Backends {
	return &Backends{
		cloud:          cloud,
		compositeCloud: composite.NewCloud(cloud),
		namer:          namer,
	}
}

// ensureDescription updates the BackendService Description with the expected value
func ensureDescription(be *composite.BackendService, sp *utils.ServicePort) (needsUpdate bool) {
	desc := sp.GetDescription()
	features.SetDescription(&desc, sp)
	descString := desc.String()
	if be.Description == descString {
		return false
	}
	be.Description = descString
	return true
}

// Create implements Pool.
func (b *Backends) Create(sp utils.ServicePort, hcLink string) (*composite.BackendService, error) {
	name := sp.BackendName(b.namer)
	namedPort := &compute.NamedPort{
		Name: b.namer.NamedPort(sp.NodePort),
		Port: sp.NodePort,
	}

	version := features.VersionFromServicePort(&sp)
	key := b.compositeCloud.CreateKey(name, sp.ILBEnabled)
	be := &composite.BackendService{
		Version:      version,
		Name:         name,
		Protocol:     string(sp.Protocol),
		Port:         namedPort.Port,
		PortName:     namedPort.Name,
		HealthChecks: []string{hcLink},
		Region:       key.Region,
	}

	if sp.ILBEnabled {
		be.LoadBalancingScheme = "INTERNAL"
	}

	ensureDescription(be, &sp)
	if err := b.compositeCloud.CreateBackendService(be, key); err != nil {
		return nil, err
	}
	// Note: We need to perform a GCE call to re-fetch the object we just created
	// so that the "Fingerprint" field is filled in. This is needed to update the
	// object without error.
	return b.Get(name, version, sp.ILBEnabled)
}

// Update implements Pool.
func (b *Backends) Update(be *composite.BackendService) error {
	// Ensure the backend service has the proper version before updating.
	be.Version = features.VersionFromDescription(be.Description)
	isRegional := be.ResourceType == meta.Regional
	if err := b.compositeCloud.UpdateBackendService(be, b.compositeCloud.CreateKey(be.Name, isRegional)); err != nil {
		return err
	}
	return nil
}

// Get implements Pool.
func (b *Backends) Get(name string, version meta.Version, regional bool) (*composite.BackendService, error) {
	be, err := b.compositeCloud.GetBackendService(version, b.compositeCloud.CreateKey(name, regional))
	if err != nil {
		return nil, err
	}
	// Evaluate the existing features from description to see if a lower
	// API version is required so that we don't lose information from
	// the existing backend service.
	versionRequired := features.VersionFromDescription(be.Description)

	if features.IsLowerVersion(versionRequired, version) {
		be, err = b.compositeCloud.GetBackendService(versionRequired, b.compositeCloud.CreateKey(name, regional))
		if err != nil {
			return nil, err
		}
	}
	return be, nil
}

// Delete implements Pool.
func (b *Backends) Delete(name string, regional bool) (err error) {
	defer func() {
		if utils.IsHTTPErrorCode(err, http.StatusNotFound) {
			err = nil
		}
	}()

	klog.V(2).Infof("Deleting backend service %v", name)

	var version meta.Version
	if regional {
		version = lbfeatures.ILBVersion
	} else {
		version = meta.VersionGA
	}

	// Try deleting health checks even if a backend is not found.
	// Use GA version since deleting should still work
	if err = b.compositeCloud.DeleteBackendService(version, b.compositeCloud.CreateKey(name, regional)); err != nil && !utils.IsHTTPErrorCode(err, http.StatusNotFound) {
		return err
	}
	return
}

// Health implements Pool.
func (b *Backends) Health(name string, version meta.Version, regional bool) string {
	be, err := b.Get(name, version, regional)
	if err != nil || len(be.Backends) == 0 {
		return "Unknown"
	}

	// TODO: Look at more than one backend's status
	// TODO: Include port, ip in the status, since it's in the health info.
	// TODO (shance) convert to composite types
	var hs *compute.BackendServiceGroupHealth
	if regional {
		hs, err = b.cloud.GetRegionalBackendServiceHealth(name, b.cloud.Region(), be.Backends[0].Group)
	} else {
		hs, err = b.cloud.GetGlobalBackendServiceHealth(name, be.Backends[0].Group)
	}
	if err != nil || len(hs.HealthStatus) == 0 || hs.HealthStatus[0] == nil {
		return "Unknown"
	}
	// TODO: State transition are important, not just the latest.
	return hs.HealthStatus[0].HealthState
}

// List lists all backends managed by this controller.
// TODO: (shance) convert to composite types
func (b *Backends) List() ([]*composite.BackendService, error) {
	// TODO: for consistency with the rest of this sub-package this method
	// should return a list of backend ports.
	backends, err := b.compositeCloud.ListAllBackendServices()
	if err != nil {
		return nil, err
	}

	var clusterBackends []*composite.BackendService

	for _, bs := range backends {
		if b.namer.NameBelongsToCluster(bs.Name) {
			clusterBackends = append(clusterBackends, bs)
		}
	}
	return clusterBackends, nil
}
