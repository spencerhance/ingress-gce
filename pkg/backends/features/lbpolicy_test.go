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
	"k8s.io/ingress-gce/pkg/apis/backendconfig/v1beta1"
	"k8s.io/ingress-gce/pkg/composite"
	"k8s.io/ingress-gce/pkg/utils"
	"testing"
)

func TestEnsureLocalityLbPolicy(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc           string
		sp             utils.ServicePort
		bs             *composite.BackendService
		updateExpected bool
	}{
		{
			desc:           "Empty policy",
			sp:             utils.ServicePort{BackendConfig: &v1beta1.BackendConfig{}},
			bs:             &composite.BackendService{},
			updateExpected: false,
		},
		{
			desc: "Policy set with empty backend service",
			sp: utils.ServicePort{
				L7ILBEnabled: true,
				BackendConfig: &v1beta1.BackendConfig{
					Spec: v1beta1.BackendConfigSpec{
						TrafficManagement: &v1beta1.TrafficManagementConfig{LocalityLbPolicy: "ROUND_ROBIN"},
					},
				},
			},
			bs:             &composite.BackendService{},
			updateExpected: true,
		},
		{
			desc: "Policy set with up to date backend service",
			sp: utils.ServicePort{
				L7ILBEnabled: true,
				BackendConfig: &v1beta1.BackendConfig{
					Spec: v1beta1.BackendConfigSpec{
						TrafficManagement: &v1beta1.TrafficManagementConfig{LocalityLbPolicy: "ROUND_ROBIN"},
					},
				},
			},
			bs: &composite.BackendService{
				LocalityLbPolicy: "ROUND_ROBIN",
			},
			updateExpected: false,
		},
		{
			desc: "Policy set with out of date backend service",
			sp: utils.ServicePort{
				L7ILBEnabled: true,
				BackendConfig: &v1beta1.BackendConfig{
					Spec: v1beta1.BackendConfigSpec{
						TrafficManagement: &v1beta1.TrafficManagementConfig{LocalityLbPolicy: "ROUND_ROBIN"},
					},
				},
			},
			bs: &composite.BackendService{
				LocalityLbPolicy: "MAGLEV",
			},
			updateExpected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			result := EnsureLocalityLbPolicy(tc.sp, tc.bs)
			if result != tc.updateExpected {
				t.Errorf("%v: expected %v but got %v", tc.desc, tc.updateExpected, result)
			}
		})
	}
}
