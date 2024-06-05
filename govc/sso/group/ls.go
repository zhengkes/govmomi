/*
Copyright (c) 2019 VMware, Inc. All Rights Reserved.

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

package group

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

	search string
}

func init() {
	cli.Register("sso.group.ls", &ls{})
}

func (cmd *ls) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.OutputFlag.Register(ctx, f)

	f.StringVar(&cmd.search, "search", "", "Search")
}

func (cmd *ls) Usage() string {
	return "[NAME]"
}

func (cmd *ls) Description() string {
	return `List SSO groups.

Examples:
  govc sso.group.ls
  govc sso.group.ls group-name # list groups in group-name
  govc sso.group.ls -search Admin # search for groups`
}

func (cmd *ls) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return cmd.OutputFlag.Process(ctx)
}

type groupResult []types.AdminGroup

func (r groupResult) Dump() interface{} {
	return []types.AdminGroup(r)
}

func (r groupResult) Write(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)
	for _, info := range r {
		fmt.Fprintf(tw, "%s\t%s\n", info.Id.Name, info.Details.Description)
	}
	return tw.Flush()
}

func (cmd *ls) Run(ctx context.Context, f *flag.FlagSet) error {
	return sso.WithClient(ctx, cmd.ClientFlag, func(c *ssoadmin.Client) error {
		if f.NArg() == 0 {
			info, err := c.FindGroups(ctx, cmd.search)
			if err != nil {
				return err
			}
			return cmd.WriteResult(groupResult(info))
		}
		info, err := c.FindGroupsInGroup(ctx, f.Arg(0), cmd.search)
		if err != nil {
			return err
		}

		return cmd.WriteResult(groupResult(info))
	})
}
