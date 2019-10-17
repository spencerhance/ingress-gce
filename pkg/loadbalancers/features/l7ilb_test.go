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
	"context"
	"testing"

	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud"
	"github.com/GoogleCloudPlatform/k8s-cloud-provider/pkg/cloud/meta"
	computebeta "google.golang.org/api/compute/v0.beta"
	"k8s.io/legacy-cloud-providers/gce"
)

// newSubnet() is a helper for creating new ILB subnets
func newSubnet(name, cidr, network, projectID string) *computebeta.Subnetwork {

	// TODO(shance): Update this once FakeGCE has a non-empty networkURL
	var networkURL string
	if network != "" {
		networkID := cloud.ResourceID{ProjectID: projectID, Resource: "networks", Key: meta.GlobalKey(network)}
		networkURL = networkID.SelfLink(meta.VersionGA)
	}

	return &computebeta.Subnetwork{
		Name:        name,
		Network:     networkURL,
		IpCidrRange: cidr,
		Role:        "ACTIVE",
		Purpose:     "INTERNAL_HTTPS_LOAD_BALANCER",
	}
}

func TestILBSubnetSourceRange(t *testing.T) {
	t.Parallel()
	defaults := gce.DefaultTestClusterValues()

	testCases := []struct {
		desc string
		// These slices must be the same length
		subnets []*computebeta.Subnetwork
		keys    []*meta.Key
		results []string
		errors  []error
	}{
		{
			desc: "Test one subnet",
			subnets: []*computebeta.Subnetwork{
				newSubnet("subnet-1", "10.0.0.0/24", "", defaults.ProjectID),
			},
			keys: []*meta.Key{
				meta.RegionalKey("subnet-1", defaults.Region),
			},
			results: []string{
				"10.0.0.0/24",
			},
			errors: []error{
				nil,
			},
		},
		{
			desc: "Two subnets, different regions",
			subnets: []*computebeta.Subnetwork{
				newSubnet("subnet-1", "10.0.0.0/24", "", defaults.ProjectID),
				newSubnet("subnet-2", "10.1.2.3/24", "default", defaults.ProjectID),
			},
			keys: []*meta.Key{
				meta.RegionalKey("subnet-1", defaults.Region),
				meta.RegionalKey("subnet-2", "us-west1"),
			},
			results: []string{
				"10.0.0.0/24",
				"10.0.0.0/24",
			},
			errors: []error{
				nil,
				nil,
			},
		},
		{
			desc: "Two subnets, different VPCs",
			subnets: []*computebeta.Subnetwork{
				newSubnet("subnet-z", "10.10.0.0/24", "", defaults.ProjectID),
				newSubnet("subnet-a", "10.1.2.3/24", "other-network", defaults.ProjectID),
			},
			keys: []*meta.Key{
				meta.RegionalKey("subnet-z", defaults.Region),
				meta.RegionalKey("subnet-a", defaults.Region),
			},
			results: []string{
				"10.10.0.0/24",
				"10.10.0.0/24",
			},
			errors: []error{
				nil,
				nil,
			},
		},
		{
			desc: "No subnet in region",
			subnets: []*computebeta.Subnetwork{
				newSubnet("subnet-1", "10.0.0.0/24", "", defaults.ProjectID),
			},
			keys: []*meta.Key{
				meta.RegionalKey("subnet-1", "us-west1"),
			},
			results: []string{
				"",
			},
			errors: []error{
				ErrSubnetNotFound,
			},
		},
		{
			desc: "No subnet in VPC",
			subnets: []*computebeta.Subnetwork{
				newSubnet("subnet-1", "10.0.0.0/24", "other-network", defaults.ProjectID),
			},
			keys: []*meta.Key{
				meta.RegionalKey("subnet-1", defaults.Region),
			},
			results: []string{
				"",
			},
			errors: []error{
				ErrSubnetNotFound,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			fakeGCE := gce.NewFakeGCECloud(gce.DefaultTestClusterValues())

			ctx := context.Background()
			for i := 0; i < len(tc.subnets); i++ {
				if err := fakeGCE.Compute().BetaSubnetworks().Insert(ctx, tc.keys[i], tc.subnets[i]); err != nil {
					t.Fatalf("Error creating subnet %v: %v", tc.subnets[i], err)
				}

				result, err := ILBSubnetSourceRange(fakeGCE, fakeGCE.Region())
				if err != nil && err != tc.errors[i] {
					t.Fatalf("got %v, want nil", err)
				}
				if result != tc.results[i] {
					t.Fatalf("want %q, got %q", tc.results[i], result)
				}
			}
		})
	}
}
