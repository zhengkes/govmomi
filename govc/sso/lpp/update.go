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

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/govc/sso"
	"github.com/zhengkes/govmomi/ssoadmin"
	"github.com/zhengkes/govmomi/ssoadmin/types"
)

type policyDetails struct {
	*flags.ClientFlag

	pol                  types.AdminPasswordPolicy
	MinLength            *int32
	MinAlphabeticCount   *int32
	MinUppercaseCount    *int32
	MinLowercaseCount    *int32
	MinNumericCount      *int32
	MinSpecialCharCount  *int32
	PasswordLifetimeDays *int32
}

func (cmd *policyDetails) Usage() string {
	return "NAME"
}

func (cmd *policyDetails) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	f.StringVar(&cmd.pol.Description, "Description", "", "Description")
	f.Var(flags.NewOptionalInt32(&cmd.MinLength), "MinLength", "Minimim length")
	f.Var(flags.NewInt32(&cmd.pol.PasswordFormat.LengthRestriction.MaxLength), "MaxLength", "Maximum length")
	f.Var(flags.NewOptionalInt32(&cmd.MinAlphabeticCount), "MinAlphabeticCount", "Minimum alphabetic count")
	f.Var(flags.NewOptionalInt32(&cmd.MinUppercaseCount), "MinUppercaseCount", "Minimum uppercase count")
	f.Var(flags.NewOptionalInt32(&cmd.MinLowercaseCount), "MinLowercaseCount", "Minimum lowercase count")
	f.Var(flags.NewOptionalInt32(&cmd.MinNumericCount), "MinNumericCount", "Minimum numeric count")
	f.Var(flags.NewOptionalInt32(&cmd.MinSpecialCharCount), "MinSpecialCharCount", "Minimum special characters count")
	f.Var(flags.NewInt32(&cmd.pol.PasswordFormat.MaxIdenticalAdjacentCharacters), "MaxIdenticalAdjacentCharacters", "Maximum identical adjacent characters")
	f.Var(flags.NewInt32(&cmd.pol.ProhibitedPreviousPasswordsCount), "ProhibitedPreviousPasswordsCount", "Prohibited previous passwords count")
	f.Var(flags.NewOptionalInt32(&cmd.PasswordLifetimeDays), "PasswordLifetimeDays", "Password lifetime days")
}

type update struct {
	policyDetails
}

func init() {
	cli.Register("sso.lpp.update", &update{})
}

func (cmd *update) Description() string {
	return `Update SSO local password policy.

Examples:
  govc sso.lpp.update -PasswordLifetimeDays 0`
}

func smerge(src *string, current string) {
	if *src == "" {
		*src = current
	}
}

func imerge(src *int32, current int32) {
	if *src == 0 {
		*src = current
	}
}

func oimerge(src *int32, flag *int32, current int32) {
	if flag == nil {
		*src = current
	} else {
		*src = *flag
	}
}

func (cmd *update) Run(ctx context.Context, f *flag.FlagSet) error {
	return sso.WithClient(ctx, cmd.ClientFlag, func(c *ssoadmin.Client) error {
		current, err := c.GetLocalPasswordPolicy(ctx)
		if err != nil {
			return err
		}

		smerge(&cmd.pol.Description, current.Description)
		oimerge(&cmd.pol.PasswordFormat.LengthRestriction.MinLength, cmd.MinLength, current.PasswordFormat.LengthRestriction.MinLength)
		imerge(&cmd.pol.PasswordFormat.LengthRestriction.MaxLength, current.PasswordFormat.LengthRestriction.MaxLength)
		oimerge(&cmd.pol.PasswordFormat.AlphabeticRestriction.MinAlphabeticCount, cmd.MinAlphabeticCount, current.PasswordFormat.AlphabeticRestriction.MinAlphabeticCount)
		oimerge(&cmd.pol.PasswordFormat.AlphabeticRestriction.MinUppercaseCount, cmd.MinUppercaseCount, current.PasswordFormat.AlphabeticRestriction.MinUppercaseCount)
		oimerge(&cmd.pol.PasswordFormat.AlphabeticRestriction.MinLowercaseCount, cmd.MinLowercaseCount, current.PasswordFormat.AlphabeticRestriction.MinLowercaseCount)
		oimerge(&cmd.pol.PasswordFormat.MinNumericCount, cmd.MinNumericCount, current.PasswordFormat.MinNumericCount)
		oimerge(&cmd.pol.PasswordFormat.MinSpecialCharCount, cmd.MinSpecialCharCount, current.PasswordFormat.MinSpecialCharCount)
		imerge(&cmd.pol.PasswordFormat.MaxIdenticalAdjacentCharacters, current.PasswordFormat.MaxIdenticalAdjacentCharacters)
		imerge(&cmd.pol.ProhibitedPreviousPasswordsCount, current.ProhibitedPreviousPasswordsCount)
		oimerge(&cmd.pol.PasswordLifetimeDays, cmd.PasswordLifetimeDays, current.PasswordLifetimeDays)

		return c.UpdateLocalPasswordPolicy(ctx, cmd.pol)
	})
}
