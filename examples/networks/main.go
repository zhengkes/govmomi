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

package main

import (
	"context"
	"fmt"

	"github.com/zhengkes/govmomi/examples"
	"github.com/zhengkes/govmomi/view"
	"github.com/zhengkes/govmomi/vim25"
	"github.com/zhengkes/govmomi/vim25/mo"
)

func main() {
	examples.Run(func(ctx context.Context, c *vim25.Client) error {
		// Create a view of Network types
		m := view.NewManager(c)

		v, err := m.CreateContainerView(ctx, c.ServiceContent.RootFolder, []string{"Network"}, true)
		if err != nil {
			return err
		}

		defer v.Destroy(ctx)

		// Reference: http://pubs.vmware.com/vsphere-60/topic/com.vmware.wssdk.apiref.doc/vim.Network.html
		var networks []mo.Network
		err = v.Retrieve(ctx, []string{"Network"}, nil, &networks)
		if err != nil {
			return err
		}

		for _, net := range networks {
			fmt.Printf("%s: %s\n", net.Name, net.Reference())
		}

		return nil
	})
}
