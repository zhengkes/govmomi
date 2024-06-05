/*
Copyright (c) 2018 VMware, Inc. All Rights Reserved.

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

package user

import (
	"context"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/govc/sso"
	"github.com/zhengkes/govmomi/ssoadmin"
	"github.com/zhengkes/govmomi/ssoadmin/types"
)

type ls struct {
	*flags.ClientFlag
	*flags.OutputFlag

	solution bool
	group    bool
	search   string
}

func init() {
	cli.Register("sso.user.ls", &ls{})
}

func (cmd *ls) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.OutputFlag.Register(ctx, f)

	f.BoolVar(&cmd.solution, "s", false, "List solution users")
	f.BoolVar(&cmd.group, "group", false, "List users in group")
	f.StringVar(&cmd.search, "search", "", "Search users in group")
}

func (cmd *ls) Description() string {
	return `List SSO users.

Examples:
  govc sso.user.ls -s
  govc sso.user.ls -group group-name`
}

func (cmd *ls) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return cmd.OutputFlag.Process(ctx)
}

type userResult []types.AdminUser

func (r userResult) Dump() interface{} {
	return []types.AdminUser(r)
}

func (r userResult) Write(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	for _, info := range r {
		fmt.Fprintf(tw, "%s\t%s\n", info.Id.Name, info.Description)
	}
	return tw.Flush()
}

type solutionResult []types.AdminSolutionUser

func (r solutionResult) Dump() interface{} {
	return []types.AdminSolutionUser(r)
}

func (r solutionResult) Write(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	for _, info := range r {
		fmt.Fprintf(tw, "%s\t%s\n", info.Id.Name, info.Details.Description)
	}
	return tw.Flush()
}

type personResult []types.AdminPersonUser

func (r personResult) Dump() interface{} {
	return []types.AdminPersonUser(r)
}

func (r personResult) Write(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	for _, info := range r {
		fmt.Fprintf(tw, "%s\t%s\n", info.Id.Name, info.Details.Description)
	}
	return tw.Flush()
}

func (cmd *ls) Run(ctx context.Context, f *flag.FlagSet) error {
	arg := f.Arg(0)

	return sso.WithClient(ctx, cmd.ClientFlag, func(c *ssoadmin.Client) error {
		if cmd.solution {
			if f.NArg() != 0 {
				return flag.ErrHelp
			}
			info, err := c.FindSolutionUsers(ctx, arg)
			if err != nil {
				return err
			}

			return cmd.WriteResult(solutionResult(info))
		}
		if cmd.group {
			if f.NArg() == 0 {
				return flag.ErrHelp
			}
			info, err := c.FindUsersInGroup(ctx, f.Arg(0), cmd.search)
			if err != nil {
				return err
			}

			return cmd.WriteResult(userResult(info))
		}
		info, err := c.FindPersonUsers(ctx, arg)
		if err != nil {
			return err
		}

		return cmd.WriteResult(personResult(info))
	})
}
