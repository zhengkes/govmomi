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

package view

import (
	"context"

	"github.com/zhengkes/govmomi/object"
	"github.com/zhengkes/govmomi/vim25"
	"github.com/zhengkes/govmomi/vim25/methods"
	"github.com/zhengkes/govmomi/vim25/types"
)

type ManagedObjectView struct {
	object.Common
}

func NewManagedObjectView(c *vim25.Client, ref types.ManagedObjectReference) *ManagedObjectView {
	return &ManagedObjectView{
		Common: object.NewCommon(c, ref),
	}
}

func (v *ManagedObjectView) TraversalSpec() *types.TraversalSpec {
	return &types.TraversalSpec{
		Path: "view",
		Type: v.Reference().Type,
	}
}

func (v *ManagedObjectView) Destroy(ctx context.Context) error {
	req := types.DestroyView{
		This: v.Reference(),
	}

	_, err := methods.DestroyView(ctx, v.Client(), &req)
	return err
}
