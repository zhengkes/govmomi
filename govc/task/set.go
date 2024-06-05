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

package task

import (
	"context"
	"flag"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/object"
	"github.com/zhengkes/govmomi/vim25/types"
)

type set struct {
	*flags.ClientFlag

	desc     types.LocalizableMessage
	state    string
	err      string
	progress int
}

func init() {
	cli.Register("task.set", &set{}, true)
}

func (cmd *set) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	f.StringVar(&cmd.desc.Key, "d", "", "Task description key")
	f.StringVar(&cmd.desc.Message, "m", "", "Task description message")
	f.StringVar(&cmd.state, "s", "", "Task state")
	f.StringVar(&cmd.err, "e", "", "Task error")
	f.IntVar(&cmd.progress, "p", 0, "Task progress")
}

func (cmd *set) Description() string {
	return `Set task state.

Examples:
  id=$(govc task.create com.vmware.govmomi.simulator.test)
  govc task.set $id -s error`
}

func (cmd *set) Usage() string {
	return "ID"
}

func (cmd *set) Run(ctx context.Context, f *flag.FlagSet) error {
	if f.NArg() != 1 {
		return flag.ErrHelp
	}

	c, err := cmd.Client()
	if err != nil {
		return err
	}

	ref := types.ManagedObjectReference{Type: "Task"}
	if !ref.FromString(f.Arg(0)) {
		ref.Value = f.Arg(0)
	}

	task := object.NewTask(c, ref)

	var fault *types.LocalizedMethodFault

	if cmd.err != "" {
		fault = &types.LocalizedMethodFault{
			Fault:            &types.SystemError{Reason: cmd.err},
			LocalizedMessage: cmd.err,
		}
		cmd.state = string(types.TaskInfoStateError)
	}

	if cmd.state != "" {
		err := task.SetState(ctx, types.TaskInfoState(cmd.state), nil, fault)
		if err != nil {
			return err
		}
	}

	if cmd.progress != 0 {
		err := task.UpdateProgress(ctx, cmd.progress)
		if err != nil {
			return err
		}
	}

	if cmd.desc.Key != "" {
		err := task.SetDescription(ctx, cmd.desc)
		if err != nil {
			return err
		}
	}

	return nil
}
