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

package idp

import (
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
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
}

func init() {
	cli.Register("sso.idp.ls", &ls{})
}

func (cmd *ls) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.OutputFlag.Register(ctx, f)
}

func (cmd *ls) Description() string {
	return `List SSO identity provider sources.

Examples:
  govc sso.idp.ls
  govc sso.idp.ls -json`
}

func (cmd *ls) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return cmd.OutputFlag.Process(ctx)
}

type idpInfo struct {
	IdentitySources *types.IdentitySources
}

func (r *idpInfo) Write(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)

	type entry struct {
		name   string
		url    string
		kind   string
		domain string
		alias  string
	}

	var entries []entry

	for _, domain := range r.IdentitySources.System.Domains {
		entries = append(entries, entry{"-", "-", "System Domain", domain.Name, domain.Alias})
	}

	if r.IdentitySources.LocalOS != nil {
		for _, domain := range r.IdentitySources.LocalOS.Domains {
			entries = append(entries, entry{"-", "-", "Local OS", domain.Name, domain.Alias})
		}
	}

	if ad := r.IdentitySources.NativeAD; ad != nil {
		for _, domain := range ad.Domains {
			entries = append(entries, entry{ad.Name, "-", "Active Directory", domain.Name, domain.Alias})
		}
	}

	for _, ldap := range r.IdentitySources.LDAPS {
		for _, domain := range ldap.Domains {
			entries = append(entries, entry{ldap.Name, ldap.Details.PrimaryURL, ldap.Type, domain.Name, domain.Alias})
		}
	}

	fmt.Fprintln(tw, "Name\tServer URL\tType\tDomain\tAlias")

	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", e.name, e.url, e.kind, strings.ToLower(e.domain), e.alias)
	}

	return tw.Flush()
}

func (cmd *ls) Run(ctx context.Context, f *flag.FlagSet) error {
	return sso.WithClient(ctx, cmd.ClientFlag, func(c *ssoadmin.Client) error {
		sources, err := c.IdentitySources(ctx)
		if err != nil {
			return err
		}

		return cmd.WriteResult(&idpInfo{sources})
	})
}
