/*
Copyright (c) 2017 VMware, Inc. All Rights Reserved.

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

package object

import (
	"context"
	"fmt"

	"github.com/zhengkes/govmomi/vim25"
	"github.com/zhengkes/govmomi/vim25/mo"
	"github.com/zhengkes/govmomi/vim25/types"
)

type OpaqueNetwork struct {
	Common
}

func NewOpaqueNetwork(c *vim25.Client, ref types.ManagedObjectReference) *OpaqueNetwork {
	return &OpaqueNetwork{
		Common: NewCommon(c, ref),
	}
}

func (n OpaqueNetwork) GetInventoryPath() string {
	return n.InventoryPath
}

// EthernetCardBackingInfo returns the VirtualDeviceBackingInfo for this Network
func (n OpaqueNetwork) EthernetCardBackingInfo(ctx context.Context) (types.BaseVirtualDeviceBackingInfo, error) {
	summary, err := n.Summary(ctx)
	if err != nil {
		return nil, err
	}

	backing := &types.VirtualEthernetCardOpaqueNetworkBackingInfo{
		OpaqueNetworkId:   summary.OpaqueNetworkId,
		OpaqueNetworkType: summary.OpaqueNetworkType,
	}

	return backing, nil
}

// Summary returns the mo.OpaqueNetwork.Summary property
func (n OpaqueNetwork) Summary(ctx context.Context) (*types.OpaqueNetworkSummary, error) {
	var props mo.OpaqueNetwork

	err := n.Properties(ctx, n.Reference(), []string{"summary"}, &props)
	if err != nil {
		return nil, err
	}

	summary, ok := props.Summary.(*types.OpaqueNetworkSummary)
	if !ok {
		return nil, fmt.Errorf("%s unsupported network summary type: %T", n, props.Summary)
	}

	return summary, nil
}
