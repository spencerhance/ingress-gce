/*
Copyright 2019 The Kubernetes Authors.

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

package features

import (
	"k8s.io/ingress-gce/pkg/composite"
	"k8s.io/ingress-gce/pkg/utils"
	"k8s.io/klog"
)

// EnsureLocalityLbPolicy Ensures that the localityLbPolicy on the BackendConfig is applied to the BackendService
func EnsureLocalityLbPolicy(sp utils.ServicePort, be *composite.BackendService) bool {
	if sp.BackendConfig.Spec.TrafficManagement == nil || sp.BackendConfig.Spec.TrafficManagement.LocalityLbPolicy == "" {
		return false
	}

	if be.LocalityLbPolicy != sp.BackendConfig.Spec.TrafficManagement.LocalityLbPolicy {
		applyLocalityLbPolicy(sp, be)
		klog.V(2).Infof("Updated LocalityLbPolicy settings for service %v/%v.", sp.ID.Service.Namespace, sp.ID.Service.Name)
		return true
	}
	return false
}

// applyLocalityLbPolicy applies the localityLbPolicy on the BackendConfig to the BackendService
func applyLocalityLbPolicy(sp utils.ServicePort, be *composite.BackendService) {
	be.LocalityLbPolicy = sp.BackendConfig.Spec.TrafficManagement.LocalityLbPolicy
}
