/*
Copyright (c) 2016-2023 VMware, Inc. All Rights Reserved.

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

package datacenter

import (
	"context"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/zhengkes/govmomi/find"
	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/object"
	"github.com/zhengkes/govmomi/property"
	"github.com/zhengkes/govmomi/view"
	"github.com/zhengkes/govmomi/vim25"
	"github.com/zhengkes/govmomi/vim25/mo"
	"github.com/zhengkes/govmomi/vim25/types"
)

type info struct {
	*flags.ClientFlag
	*flags.OutputFlag
	*flags.DatacenterFlag
}

func init() {
	cli.Register("datacenter.info", &info{})
}

func (cmd *info) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.OutputFlag.Register(ctx, f)

	cmd.DatacenterFlag, ctx = flags.NewDatacenterFlag(ctx)
	cmd.DatacenterFlag.Register(ctx, f)
}

func (cmd *info) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	if err := cmd.OutputFlag.Process(ctx); err != nil {
		return err
	}
	if err := cmd.DatacenterFlag.Process(ctx); err != nil {
		return err
	}
	return nil
}

func (cmd *info) Usage() string {
	return "[PATH]..."
}

func (cmd *info) Run(ctx context.Context, f *flag.FlagSet) error {
	c, err := cmd.Client()
	if err != nil {
		return err
	}

	finder, err := cmd.Finder()
	if err != nil {
		return err
	}

	args := f.Args()
	if len(args) == 0 {
		args = []string{"*"}
	}

	var props []string
	res := infoResult{
		finder: finder,
		ctx:    ctx,
		client: c,
	}

	if !cmd.OutputFlag.All() {
		props = []string{
			"name",
			"vmFolder",
			"hostFolder",
			"datastoreFolder",
			"networkFolder",
			"datastore",
			"network",
		}
	}

	for _, arg := range args {
		objects, err := finder.DatacenterList(ctx, arg)
		if err != nil {
			return err
		}
		res.objects = append(res.objects, objects...)
	}

	if len(res.objects) != 0 {
		refs := make([]types.ManagedObjectReference, 0, len(res.objects))
		for _, o := range res.objects {
			refs = append(refs, o.Reference())
		}

		pc := property.DefaultCollector(c)
		err = pc.Retrieve(ctx, refs, props, &res.Datacenters)
		if err != nil {
			return err
		}
	}

	return cmd.WriteResult(&res)
}

type infoResult struct {
	Datacenters []mo.Datacenter `json:"datacenters"`
	objects     []*object.Datacenter
	finder      *find.Finder
	ctx         context.Context
	client      *vim25.Client
}

func (r *infoResult) Write(w io.Writer) error {
	// Maintain order via r.objects as Property collector does not always return results in order.
	objects := make(map[types.ManagedObjectReference]mo.Datacenter, len(r.Datacenters))
	for _, o := range r.Datacenters {
		objects[o.Reference()] = o
	}

	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)

	for _, o := range r.objects {
		dc := objects[o.Reference()]
		fmt.Fprintf(tw, "Name:\t%s\n", dc.Name)
		fmt.Fprintf(tw, "  Path:\t%s\n", o.InventoryPath)

		r.finder.SetDatacenter(o)

		hosts, _ := r.finder.HostSystemList(r.ctx, "*")
		fmt.Fprintf(tw, "  Hosts:\t%d\n", len(hosts))

		clusters, _ := r.finder.ClusterComputeResourceList(r.ctx, "*")
		fmt.Fprintf(tw, "  Clusters:\t%d\n", len(clusters))

		manager := view.NewManager(r.client)

		v, err := manager.CreateContainerView(r.ctx, o.Reference(), []string{"VirtualMachine"}, true)
		if err != nil {
			return err
		}

		var vms []mo.VirtualMachine
		err = v.Retrieve(r.ctx, []string{"VirtualMachine"}, []string{"summary.config.template"}, &vms)
		if err != nil {
			return err
		}

		defer func() {
			_ = v.Destroy(r.ctx)
		}()

		totalVms := 0
		for _, vm := range vms {
			if vm.Summary.Config.Template {
				continue
			}
			totalVms++
		}

		fmt.Fprintf(tw, "  Virtual Machines:\t%d\n", totalVms)

		fmt.Fprintf(tw, "  Networks:\t%d\n", len(dc.Network))
		fmt.Fprintf(tw, "  Datastores:\t%d\n", len(dc.Datastore))
	}

	return tw.Flush()
}
