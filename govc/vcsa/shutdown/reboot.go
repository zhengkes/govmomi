/*
Copyright (c) 2022 VMware, Inc. All Rights Reserved.

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

package shutdown

import (
	"context"
	"flag"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/vapi/appliance/shutdown"
)

type reboot struct {
	*flags.ClientFlag

	reason string
	delay  int // in minutes
}

func init() {
	cli.Register("vcsa.shutdown.reboot", &reboot{})
}

func (cmd *reboot) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	var now int
	f.IntVar(&cmd.delay,
		"delay",
		now,
		"Minutes after which reboot should start.")
}

func (cmd *reboot) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return nil
}

func (cmd *reboot) Usage() string {
	return "REASON"
}

func (cmd *reboot) Description() string {
	return `Reboot the appliance.

Note: This command requires vCenter 7.0.2 or higher.

Examples:
govc vcsa.shutdown.reboot -delay 10 "rebooting for maintenance"`
}

func (cmd *reboot) Run(ctx context.Context, f *flag.FlagSet) error {
	if f.NArg() != 1 {
		return flag.ErrHelp
	}

	c, err := cmd.RestClient()
	if err != nil {
		return err
	}

	m := shutdown.NewManager(c)

	if err = m.Reboot(ctx, f.Arg(0), cmd.delay); err != nil {
		return err
	}

	return nil
}
