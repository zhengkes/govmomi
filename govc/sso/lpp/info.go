/*
Copyright (c) 2022-2022 VMware, Inc. All Rights Reserved.

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

package lpp

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/govc/sso"
	"github.com/zhengkes/govmomi/ssoadmin"
	"github.com/zhengkes/govmomi/ssoadmin/types"
)

type info struct {
	*flags.ClientFlag
	*flags.OutputFlag
}

func init() {
	cli.Register("sso.lpp.info", &info{})
}

func (cmd *info) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.OutputFlag.Register(ctx, f)
}

func (cmd *info) Description() string {
	return `Get SSO local password policy.

Examples:
  govc sso.lpp.info
  govc sso.lpp.info -json`
}

func (cmd *info) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return cmd.OutputFlag.Process(ctx)
}

type lppInfo struct {
	LocalPasswordPolicy *types.AdminPasswordPolicy
}

func (r *lppInfo) Write(w io.Writer) error {
	fmt.Fprintf(
		w,
		"Description: %s\n"+
			"MinLength: %d\n"+
			"MaxLength: %d\n"+
			"MinAlphabeticCount: %d\n"+
			"MinUppercaseCount: %d\n"+
			"MinLowercaseCount: %d\n"+
			"MinNumericCount: %d\n"+
			"MinSpecialCharCount: %d\n"+
			"MaxIdenticalAdjacentCharacters: %d\n"+
			"ProhibitedPreviousPasswordsCount: %d\n"+
			"PasswordLifetimeDays: %d\n",
		r.LocalPasswordPolicy.Description,
		r.LocalPasswordPolicy.PasswordFormat.LengthRestriction.MinLength,
		r.LocalPasswordPolicy.PasswordFormat.LengthRestriction.MaxLength,
		r.LocalPasswordPolicy.PasswordFormat.AlphabeticRestriction.MinAlphabeticCount,
		r.LocalPasswordPolicy.PasswordFormat.AlphabeticRestriction.MinUppercaseCount,
		r.LocalPasswordPolicy.PasswordFormat.AlphabeticRestriction.MinLowercaseCount,
		r.LocalPasswordPolicy.PasswordFormat.MinNumericCount,
		r.LocalPasswordPolicy.PasswordFormat.MinSpecialCharCount,
		r.LocalPasswordPolicy.PasswordFormat.MaxIdenticalAdjacentCharacters,
		r.LocalPasswordPolicy.ProhibitedPreviousPasswordsCount,
		r.LocalPasswordPolicy.PasswordLifetimeDays,
	)
	return nil
}

func (r *lppInfo) Dump() interface{} {
	return r.LocalPasswordPolicy
}

func (cmd *info) Run(ctx context.Context, f *flag.FlagSet) error {
	return sso.WithClient(ctx, cmd.ClientFlag, func(c *ssoadmin.Client) error {
		var err error
		var pol lppInfo
		pol.LocalPasswordPolicy, err = c.GetLocalPasswordPolicy(ctx)
		if err != nil {
			return err
		}
		return cmd.WriteResult(&pol)
	})
}
