/*
Copyright (c) 2015 VMware, Inc. All Rights Reserved.

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
	"github.com/zhengkes/govmomi/vim25/methods"
	"github.com/zhengkes/govmomi/vim25/types"
)

type DistributedVirtualSwitch struct {
	Common
}

func NewDistributedVirtualSwitch(c *vim25.Client, ref types.ManagedObjectReference) *DistributedVirtualSwitch {
	return &DistributedVirtualSwitch{
		Common: NewCommon(c, ref),
	}
}

func (s DistributedVirtualSwitch) GetInventoryPath() string {
	return s.InventoryPath
}

func (s DistributedVirtualSwitch) EthernetCardBackingInfo(ctx context.Context) (types.BaseVirtualDeviceBackingInfo, error) {
	ref := s.Reference()
	name := s.InventoryPath
	if name == "" {
		name = ref.String()
	}
	return nil, fmt.Errorf("type %s (%s) cannot be used for EthernetCardBackingInfo", ref.Type, name)
}

func (s DistributedVirtualSwitch) Reconfigure(ctx context.Context, spec types.BaseDVSConfigSpec) (*Task, error) {
	req := types.ReconfigureDvs_Task{
		This: s.Reference(),
		Spec: spec,
	}

	res, err := methods.ReconfigureDvs_Task(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewTask(s.Client(), res.Returnval), nil
}

func (s DistributedVirtualSwitch) AddPortgroup(ctx context.Context, spec []types.DVPortgroupConfigSpec) (*Task, error) {
	req := types.AddDVPortgroup_Task{
		This: s.Reference(),
		Spec: spec,
	}

	res, err := methods.AddDVPortgroup_Task(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewTask(s.Client(), res.Returnval), nil
}

func (s DistributedVirtualSwitch) FetchDVPorts(ctx context.Context, criteria *types.DistributedVirtualSwitchPortCriteria) ([]types.DistributedVirtualPort, error) {
	req := &types.FetchDVPorts{
		This:     s.Reference(),
		Criteria: criteria,
	}

	res, err := methods.FetchDVPorts(ctx, s.Client(), req)
	if err != nil {
		return nil, err
	}
	return res.Returnval, nil
}

func (s DistributedVirtualSwitch) ReconfigureDVPort(ctx context.Context, spec []types.DVPortConfigSpec) (*Task, error) {
	req := types.ReconfigureDVPort_Task{
		This: s.Reference(),
		Port: spec,
	}

	res, err := methods.ReconfigureDVPort_Task(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewTask(s.Client(), res.Returnval), nil
}

func (s DistributedVirtualSwitch) ReconfigureLACP(ctx context.Context, spec []types.VMwareDvsLacpGroupSpec) (*Task, error) {
	req := types.UpdateDVSLacpGroupConfig_Task{
		This:          s.Reference(),
		LacpGroupSpec: spec,
	}

	res, err := methods.UpdateDVSLacpGroupConfig_Task(ctx, s.Client(), &req)
	if err != nil {
		return nil, err
	}

	return NewTask(s.Client(), res.Returnval), nil
}
