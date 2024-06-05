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
	"fmt"
	"strings"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/vapi/vm/dataset"
)

var accessModes = []string{
	string(dataset.AccessNone),
	string(dataset.AccessReadOnly),
	string(dataset.AccessReadWrite),
}

func hostAccessUsage() string {
	return fmt.Sprintf("Access to the data set entries from the ESXi host and the vCenter (%s)", strings.Join(accessModes, "|"))
}

func guestAccessUsage() string {
	return fmt.Sprintf("Access to the data set entries from the VM guest OS (%s)", strings.Join(accessModes, "|"))
}

func validateDataSetAccess(access dataset.Access) bool {
	for _, validAccess := range accessModes {
		if string(access) == validAccess {
			return true
		}
	}
	return false
}

type create struct {
	*flags.VirtualMachineFlag
	spec dataset.CreateSpec
}

func init() {
	cli.Register("vm.dataset.create", &create{})
}

func (cmd *create) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.VirtualMachineFlag, ctx = flags.NewVirtualMachineFlag(ctx)
	cmd.VirtualMachineFlag.Register(ctx, f)
	f.StringVar(&cmd.spec.Description, "d", "", "Description")
	f.StringVar((*string)(&cmd.spec.Host), "host-access", string(dataset.AccessReadWrite), hostAccessUsage())
	f.StringVar((*string)(&cmd.spec.Guest), "guest-access", string(dataset.AccessReadWrite), guestAccessUsage())
	f.Var(flags.NewOptionalBool(&cmd.spec.OmitFromSnapshotAndClone), "omit-from-snapshot", "Omit the data set from snapshots and clones of the VM (defaults to false)")
}

func (cmd *create) Process(ctx context.Context) error {
	return cmd.VirtualMachineFlag.Process(ctx)
}

func (cmd *create) Usage() string {
	return "NAME"
}

func (cmd *create) Description() string {
	return `Create data set on a VM.

This command will output the ID of the new data set.

Examples:
  govc vm.dataset.create -vm $vm -d "Data set for project 2" -host-access READ_WRITE -guest-access READ_ONLY com.example.project2
  govc vm.dataset.create -vm $vm -d "Data set for project 3" -omit-from-snapshot=false com.example.project3`
}

func (cmd *create) Run(ctx context.Context, f *flag.FlagSet) error {
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

	cmd.spec.Name = f.Arg(0)
	if !validateDataSetAccess(cmd.spec.Host) {
		return errors.New("please specify valid host access")
	}
	if !validateDataSetAccess(cmd.spec.Guest) {
		return errors.New("please specify valid guest access")
	}

	c, err := cmd.RestClient()
	if err != nil {
		return err
	}
	mgr := dataset.NewManager(c)
	id, err := mgr.CreateDataSet(ctx, vmId, &cmd.spec)
	if err != nil {
		return err
	}
	fmt.Println(id)
	return nil
}
