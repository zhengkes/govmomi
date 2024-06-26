/*
Copyright (c) 2014-2016 VMware, Inc. All Rights Reserved.

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

package guest

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/zhengkes/govmomi/vim25/types"
)

type AuthFlag struct {
	auth types.NamePasswordAuthentication
	proc bool
}

func newAuthFlag(ctx context.Context) (*AuthFlag, context.Context) {
	return &AuthFlag{}, ctx
}

func (flag *AuthFlag) String() string {
	return fmt.Sprintf("%s:%s", flag.auth.Username, strings.Repeat("x", len(flag.auth.Password)))
}

func (flag *AuthFlag) Set(s string) error {
	c := strings.SplitN(s, ":", 2)
	if len(c) > 0 {
		flag.auth.Username = c[0]
		if len(c) > 1 {
			flag.auth.Password = c[1]
		}
	}

	return nil
}

func (flag *AuthFlag) Register(ctx context.Context, f *flag.FlagSet) {
	env := "GOVC_GUEST_LOGIN"
	value := os.Getenv(env)
	err := flag.Set(value)
	if err != nil {
		fmt.Printf("could not set guest login values: %v", err)
	}
	usage := fmt.Sprintf("Guest VM credentials (<user>:<password>) [%s]", env)
	f.Var(flag, "l", usage)
	if flag.proc {
		f.BoolVar(&flag.auth.GuestAuthentication.InteractiveSession, "i", false, "Interactive session")
	}
}

func (flag *AuthFlag) Process(ctx context.Context) error {
	if flag.auth.Username == "" {
		return fmt.Errorf("guest login username must not be empty")
	}

	return nil
}

func (flag *AuthFlag) Auth() types.BaseGuestAuthentication {
	return &flag.auth
}
