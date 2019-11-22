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

package upgrade

import (
	"context"
	"testing"

	"k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/ingress-gce/pkg/e2e"
	"k8s.io/ingress-gce/pkg/fuzz"
	"k8s.io/ingress-gce/pkg/utils/common"
)

var (
	port80  = intstr.FromInt(80)
	ingName = "ing1"
)

// Finalizer implements e2e.UpgradeTest interface.
type BasicHTTP struct {
	t         *testing.T
	s         *e2e.Sandbox
	framework *e2e.Framework
	crud      e2e.IngressCRUD
	ing       *v1beta1.Ingress
}

// NewBasicHTTPUpgradeTest returns an upgrade test that tests the basic behavior
// of an ingress with http load-balancer.
func NewBasicHTTPUpgradeTest() e2e.UpgradeTest {
	return &BasicHTTP{}
}

// Name implements e2e.UpgradeTest.Init.
func (bh *BasicHTTP) Name() string {
	return "BasicHTTPUpgrade"
}

// Init implements e2e.UpgradeTest.Init.
func (bh *BasicHTTP) Init(t *testing.T, s *e2e.Sandbox, framework *e2e.Framework) error {
	bh.t = t
	bh.s = s
	bh.framework = framework
	return nil
}

// PreUpgrade implements e2e.UpgradeTest.PreUpgrade.
func (bh *BasicHTTP) PreUpgrade() error {
	_, err := e2e.CreateEchoService(bh.s, svcName, nil)
	if err != nil {
		bh.t.Fatalf("error creating echo service: %v", err)
	}
	bh.t.Logf("Echo service created (%s/%s)", bh.s.Namespace, svcName)

	ing := fuzz.NewIngressBuilder(bh.s.Namespace, ingName, "").
		AddPath("foo.com", "/", svcName, port80).
		Build()
	ingKey := common.NamespacedName(ing)
	bh.crud = e2e.IngressCRUD{C: bh.framework.Clientset}
	if _, err := bh.crud.Create(ing); err != nil {
		bh.t.Fatalf("error creating Ingress %s: %v", ingKey, err)
	}
	bh.t.Logf("Ingress created (%s)", ingKey)

	if bh.ing, err = e2e.UpgradeTestWaitForIngress(bh.s, ing, &e2e.WaitForIngressOptions{ExpectUnreachable: true}); err != nil {
		bh.t.Fatalf("error waiting for Ingress %s to stabilize: %v", ingKey, err)
	}
	bh.t.Logf("GCLB resources created (%s)", ingKey)

	if _, err := e2e.WhiteboxTest(bh.ing, bh.s, bh.framework.Cloud, ""); err != nil {
		bh.t.Fatalf("e2e.WhiteboxTest(%s, ...) = %v, want nil", ingKey, err)
	}
	return nil
}

// DuringUpgrade implements e2e.UpgradeTest.DuringUpgrade.
func (bh *BasicHTTP) DuringUpgrade() error {
	return nil
}

// PostUpgrade implements e2e.UpgradeTest.PostUpgrade
func (bh *BasicHTTP) PostUpgrade() error {
	// force ingress update. only add path once
	newIng := fuzz.NewIngressBuilderFromExisting(bh.ing).
		AddPath("bar.com", "/", svcName, port80).
		Build()
	ingKey := common.NamespacedName(newIng)
	// TODO: does the path need to be different for each upgrade
	if _, err := bh.crud.Update(newIng); err != nil {
		bh.t.Fatalf("error updating Ingress %s: %v", ingKey, err)
	} else {
		// If Ingress upgrade succeeds, we update the status on this Ingress
		// to Unstable. It is set back to Stable after WaitForIngress below
		// finishes successfully.
		bh.s.PutStatus(e2e.Unstable)
	}

	// Verify the Ingress has stabilized after the master upgrade and we
	// trigger an Ingress update
	ing, err := e2e.WaitForIngress(bh.s, bh.ing, &e2e.WaitForIngressOptions{ExpectUnreachable: true})
	if err != nil {
		bh.t.Fatalf("error waiting for Ingress %s to stabilize: %v", ingKey, err)
	}
	bh.s.PutStatus(e2e.Stable)
	bh.t.Logf("GCLB is stable (%s)", ingKey)
	gclb, err := e2e.WhiteboxTest(ing, bh.s, bh.framework.Cloud, "")
	if err != nil {
		bh.t.Fatalf("e2e.WhiteboxTest(%s, ...) = %v, want nil", ingKey, err)
	}

	// If the Master has upgraded and the Ingress is stable,
	// we delete the Ingress and exit out of the loop to indicate that
	// the test is done.
	deleteOptions := &fuzz.GCLBDeleteOptions{
		SkipDefaultBackend: true,
	}

	if err := e2e.WaitForIngressDeletion(context.Background(), gclb, bh.s, ing, deleteOptions); err != nil { // Sometimes times out waiting
		bh.t.Errorf("e2e.WaitForIngressDeletion(..., %q, nil) = %v, want nil", ingKey, err)
	}
	return nil
}
