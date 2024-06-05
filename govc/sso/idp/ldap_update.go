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
	"reflect"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/govc/sso"
	"github.com/zhengkes/govmomi/ssoadmin"
	"github.com/zhengkes/govmomi/ssoadmin/types"
)

type ldapUpdate struct {
	*flags.ClientFlag
	serverType string
	alias      string
	idpDetails types.LdapIdentitySourceDetails
	auth       types.SsoAdminIdentitySourceManagementServiceAuthenticationCredentails
}

func (cmd *ldapUpdate) Usage() string {
	return "NAME"
}

func (cmd *ldapUpdate) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	f.StringVar(&cmd.serverType, "ServerType", "ActiveDirectory", "ServerType")
	f.StringVar(&cmd.alias, "DomainAlias", "", "DomainAlias")
	f.StringVar(&cmd.idpDetails.FriendlyName, "FriendlyName", "", "FriendlyName")
	f.StringVar(&cmd.idpDetails.UserBaseDn, "UserBaseDn", "", "UserBaseDn")
	f.StringVar(&cmd.idpDetails.GroupBaseDn, "GroupBaseDn", "", "GroupBaseDn")
	f.StringVar(&cmd.idpDetails.PrimaryURL, "PrimaryUrl", "", "PrimaryUrl")
	f.StringVar(&cmd.idpDetails.FailoverURL, "FailoverUrl", "", "FailoverUrl")
	f.StringVar(&cmd.auth.Username, "AuthUsername", "", "Username")
	f.StringVar(&cmd.auth.Password, "AuthPassword", "", "Password")
}

type lidpupd struct {
	ldapUpdate
}

func init() {
	cli.Register("sso.idp.ldap.update", &lidpupd{})
}

func (cmd *lidpupd) Description() string {
	return `Update SSO ldap identity provider source.

Examples:
  govc sso.idp.ldap.update  -FriendlyName CORPLOCAL corp.local`
}

func smerge(src *string, current string) {
	if *src == "" {
		*src = current
	}
}

func (cmd *lidpupd) Run(ctx context.Context, f *flag.FlagSet) error {
	if f.NArg() != 1 {
		return flag.ErrHelp
	}
	idpname := f.Arg(0)
	return sso.WithClient(ctx, cmd.ClientFlag, func(c *ssoadmin.Client) error {
		sources, err := c.IdentitySources(ctx)
		if err != nil {
			return err
		}

		GetLdapIdentitySourceByName := func(i []types.LdapIdentitySource, name string) *types.LdapIdentitySource {
			var n []types.LdapIdentitySource
			for _, e := range i {
				if e.Name == name {
					n = append(n, e)
				}
			}
			if len(n) != 1 {
				return nil
			}
			return &n[0]
		}

		currentidp := GetLdapIdentitySourceByName(sources.LDAPS, idpname)
		if currentidp == nil {
			return c.RegisterLdap(ctx, cmd.serverType, idpname, cmd.alias, cmd.idpDetails, cmd.auth)
		}

		if cmd.auth.Username != "" && cmd.auth.Password != "" {
			updateLdapAuthnErr := c.UpdateLdapAuthnType(ctx, idpname, cmd.auth)
			if updateLdapAuthnErr != nil {
				return updateLdapAuthnErr
			}
		}

		IsAnyIdpDetails := func(d types.LdapIdentitySourceDetails) bool {
			values := reflect.ValueOf(cmd.idpDetails)
			for i := 0; i < values.NumField(); i++ {
				if values.Field(i).Interface() != "" {
					return true
				}
			}
			return false
		}
		if IsAnyIdpDetails(cmd.idpDetails) {
			smerge(&cmd.idpDetails.FriendlyName, currentidp.Details.FriendlyName)
			smerge(&cmd.idpDetails.UserBaseDn, currentidp.Details.UserBaseDn)
			smerge(&cmd.idpDetails.GroupBaseDn, currentidp.Details.GroupBaseDn)
			smerge(&cmd.idpDetails.PrimaryURL, currentidp.Details.PrimaryURL)
			smerge(&cmd.idpDetails.FailoverURL, currentidp.Details.FailoverURL)
			updateLdapErr := c.UpdateLdap(ctx, idpname, cmd.idpDetails)
			if updateLdapErr != nil {
				return updateLdapErr
			}
		}
		return nil
	})
}
