/*
Copyright (c) 2018 VMware, Inc. All Rights Reserved.

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

package simulator

import (
	"context"
	"log"
	"testing"

	"github.com/zhengkes/govmomi"
	"github.com/zhengkes/govmomi/lookup"
	"github.com/zhengkes/govmomi/lookup/types"
	"github.com/zhengkes/govmomi/simulator"
)

func TestClient(t *testing.T) {
	ctx := context.Background()

	model := simulator.VPX()

	defer model.Remove()
	err := model.Create()
	if err != nil {
		log.Fatal(err)
	}

	s := model.Service.NewServer()
	defer s.Close()

	model.Service.RegisterSDK(New())

	vc, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	c, err := lookup.NewClient(ctx, vc.Client)
	if err != nil {
		t.Fatal(err)
	}

	id, err := c.SiteID(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if id != siteID {
		t.Errorf("SiteID=%s", id)
	}

	vc.Logout(ctx) // List does not require authentication

	_, err = c.List(ctx, nil)
	if err == nil {
		t.Error("expected error")
	}

	// test filters that should return 1 service
	filters := []*types.LookupServiceRegistrationFilter{
		{
			ServiceType: &types.LookupServiceRegistrationServiceType{
				Product: "com.vmware.cis",
				Type:    "vcenterserver",
			},
			EndpointType: &types.LookupServiceRegistrationEndpointType{
				Protocol: "vmomi",
				Type:     "com.vmware.vim",
			},
		},
		{
			ServiceType: &types.LookupServiceRegistrationServiceType{
				Type: "cs.identity",
			},
			EndpointType: &types.LookupServiceRegistrationEndpointType{
				Protocol: "wsTrust",
			},
		},
		{
			ServiceType: &types.LookupServiceRegistrationServiceType{},
			EndpointType: &types.LookupServiceRegistrationEndpointType{
				Protocol: "vmomi",
				Type:     "com.vmware.vim",
			},
		},
	}

	for _, filter := range filters {
		info, err := c.List(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}

		if len(info) != 1 {
			t.Errorf("len=%d", len(info))
		}

		filter.ServiceType.Type = "enoent"

		info, err = c.List(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}

		if len(info) != 0 {
			t.Errorf("len=%d", len(info))
		}
	}

	// "empty" filters should return all services
	filters = []*types.LookupServiceRegistrationFilter{
		{},
		{
			ServiceType:  new(types.LookupServiceRegistrationServiceType),
			EndpointType: new(types.LookupServiceRegistrationEndpointType),
		},
		{
			EndpointType: new(types.LookupServiceRegistrationEndpointType),
		},
		{
			ServiceType: new(types.LookupServiceRegistrationServiceType),
		},
	}

	for _, filter := range filters {
		info, err := c.List(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}

		if len(info) != 4 {
			t.Errorf("len=%d", len(info))
		}
	}

	vc.Client.ServiceContent.Setting = nil // ensure we don't NPE without this set
	_, err = lookup.NewClient(ctx, vc.Client)
	if err != nil {
		t.Fatal(err)
	}
}
