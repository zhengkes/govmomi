/*
Copyright (c) 2023-2023 VMware, Inc. All Rights Reserved.

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

package dataset

import (
	"context"
	"errors"
	"flag"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/vapi/vm/dataset"
)

type update struct {
	*flags.VirtualMachineFlag

	description              string
	host                     dataset.Access
	guest                    dataset.Access
	omitFromSnapshotAndClone *bool
}

func init() {
	cli.Register("vm.dataset.update", &update{})
}

func FindDataSetId(ctx context.Context, mgr *dataset.Manager, vmId string, nameOrId string) (string, error) {
	l, err := mgr.ListDataSets(ctx, vmId)
	if err != nil {
		return "", err
	}
	for _, summary := range l {
		if nameOrId == summary.DataSet || nameOrId == summary.Name {
			return summary.DataSet, nil
		}
	}
	return nameOrId, nil
}

func (cmd *update) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.VirtualMachineFlag, ctx = flags.NewVirtualMachineFlag(ctx)
	cmd.VirtualMachineFlag.Register(ctx, f)
	f.StringVar(&cmd.description, "d", "", "Description")
	f.StringVar((*string)(&cmd.host), "host-access", "", hostAccessUsage())
	f.StringVar((*string)(&cmd.guest), "guest-access", "", guestAccessUsage())
	f.Var(flags.NewOptionalBool(&cmd.omitFromSnapshotAndClone), "omit-from-snapshot", "Omit the data set from snapshots and clones of the VM")
}

func (cmd *update) Process(ctx context.Context) error {
	return cmd.VirtualMachineFlag.Process(ctx)
}

func (cmd *update) Usage() string {
	return "NAME"
}

func (cmd *update) Description() string {
	return `Update data set.
	
Examples:
  govc vm.dataset.update -vm $vm -d "New description." -guest-access READ_ONLY com.example.project2
  govc vm.dataset.update -vm $vm -omit-from-snapshot=false com.example.project3`
}

func (cmd *update) Run(ctx context.Context, f *flag.FlagSet) error {
	if f.NArg() != 1 {
		return flag.ErrHelp
	}

	vm, err := cmd.VirtualMachineFlag.VirtualMachine()
	if err != nil {
		return err
	}
	if vm == nil {
		return flag.ErrHelp
	}
	vmId := vm.Reference().Value

	if cmd.host != "" && !validateDataSetAccess(cmd.host) {
		return errors.New("please specify valid host access")
	}
	if cmd.guest != "" && !validateDataSetAccess(cmd.guest) {
		return errors.New("please specify valid guest access")
	}

	c, err := cmd.RestClient()
	if err != nil {
		return err
	}
	mgr := dataset.NewManager(c)

	id, err := FindDataSetId(ctx, mgr, vmId, f.Arg(0))
	if err != nil {
		return err
	}

	// Update only the fields which the user asked for
	updateSpec := dataset.UpdateSpec{}
	if cmd.description != "" {
		updateSpec.Description = &cmd.description
	}
	if cmd.host != "" {
		updateSpec.Host = &cmd.host
	}
	if cmd.guest != "" {
		updateSpec.Guest = &cmd.guest
	}
	if cmd.omitFromSnapshotAndClone != nil {
		updateSpec.OmitFromSnapshotAndClone = cmd.omitFromSnapshotAndClone
	}

	err = mgr.UpdateDataSet(ctx, vmId, id, &updateSpec)
	if err != nil {
		return err
	}

	return nil
}
