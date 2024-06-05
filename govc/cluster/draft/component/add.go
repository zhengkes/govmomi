/*
Copyright (c) 2024-2024 VMware, Inc. All Rights Reserved.

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

package component

import (
	"context"
	"flag"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/vapi/esx/settings/clusters"
)

type add struct {
	*flags.ClientFlag
	*flags.OutputFlag

	clusterId        string
	draftId          string
	componentId      string
	componentVersion string
}

func init() {
	cli.Register("cluster.draft.component.add", &add{})
}

func (cmd *add) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)
	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)

	f.StringVar(&cmd.clusterId, "cluster-id", "", "The identifier of the cluster.")
	f.StringVar(&cmd.draftId, "draft-id", "", "The identifier of the software draft.")
	f.StringVar(&cmd.componentId, "component-id", "", "The identifier of the software component.")
	f.StringVar(&cmd.componentVersion, "component-version", "", "The version of the software component.")
}

func (cmd *add) Process(ctx context.Context) error {
	return cmd.ClientFlag.Process(ctx)
}

func (cmd *add) Usage() string {
	return "CLUSTER"
}

func (cmd *add) Description() string {
	return `Adds a new component to the software draft.  

Examples:
  govc cluster.draft.component.add -cluster-id=domain-c21 -draft-id=13 -component-id=NVD-AIE-800 -component-version=550.54.10-1OEM.800.1.0.20613240`
}

func (cmd *add) Run(ctx context.Context, f *flag.FlagSet) error {
	rc, err := cmd.RestClient()

	if err != nil {
		return err
	}

	dm := clusters.NewManager(rc)

	spec := clusters.SoftwareComponentsUpdateSpec{}
	spec.ComponentsToSet = make(map[string]string)
	spec.ComponentsToSet[cmd.componentId] = cmd.componentVersion
	return dm.UpdateSoftwareDraftComponents(cmd.clusterId, cmd.draftId, spec)
}
