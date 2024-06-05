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

package idp

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/govc/sso"
	"github.com/zhengkes/govmomi/ssoadmin"
)

type didp struct {
	*flags.ClientFlag
	*flags.OutputFlag
}

func init() {
	cli.Register("sso.idp.default.ls", &didp{})
}

func (cmd *didp) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.OutputFlag.Register(ctx, f)
}

func (cmd *didp) Description() string {
	return `List SSO default identity provider sources.

Examples:
  govc sso.idp.default.ls
  govc sso.idp.default.ls -json`
}

func (cmd *didp) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return cmd.OutputFlag.Process(ctx)
}

type didpInfo struct {
	DefaultIdentitySource []string
}

func (r *didpInfo) Write(w io.Writer) error {
	fmt.Fprintf(w, "Default identity provider source(s): %s\n", strings.Join(r.DefaultIdentitySource, ","))
	return nil
}

func (cmd *didp) Run(ctx context.Context, f *flag.FlagSet) error {
	return sso.WithClient(ctx, cmd.ClientFlag, func(c *ssoadmin.Client) error {
		var errids error
		var defaultids didpInfo

		defaultids.DefaultIdentitySource, errids = c.GetDefaultDomains(ctx)
		if errids != nil {
			return errids
		}
		return cmd.WriteResult(&defaultids)
	})
}
