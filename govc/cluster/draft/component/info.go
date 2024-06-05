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
	"io"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/vapi/esx/settings/clusters"
)

type infoResult clusters.SettingsComponentInfo

func (r infoResult) Write(w io.Writer) error {
	return nil
}

type info struct {
	*flags.ClientFlag
	*flags.OutputFlag

	clusterId   string
	draftId     string
	componentId string
}

func init() {
	cli.Register("cluster.draft.component.info", &info{})
}

func (cmd *info) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)
	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)

	f.StringVar(&cmd.clusterId, "cluster-id", "", "The identifier of the cluster.")
	f.StringVar(&cmd.draftId, "draft-id", "", "The identifier of the software draft.")
	f.StringVar(&cmd.componentId, "component-id", "", "The identifier of the software component.")
}

func (cmd *info) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	if err := cmd.OutputFlag.Process(ctx); err != nil {
		return err
	}

	return nil
}

func (cmd *info) Usage() string {
	return "CLUSTER"
}

func (cmd *info) Description() string {
	return `Displays the details of a component in a software draft.  

Examples:
  govc cluster.draft.component.info -cluster-id=domain-c21 -draft-id=13 -component-id=NVD-AIE-800`
}

func (cmd *info) Run(ctx context.Context, f *flag.FlagSet) error {
	rc, err := cmd.RestClient()

	if err != nil {
		return err
	}

	dm := clusters.NewManager(rc)

	if d, err := dm.GetSoftwareDraftComponent(cmd.clusterId, cmd.draftId, cmd.componentId); err != nil {
		return err
	} else {
		if !cmd.All() {
			cmd.JSON = true
		}
		return cmd.WriteResult(infoResult(d))
	}
}
