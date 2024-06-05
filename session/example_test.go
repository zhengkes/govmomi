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

package session_test

import (
	"context"
	"fmt"
	"net/url"

	_ "github.com/zhengkes/govmomi/lookup/simulator"
	"github.com/zhengkes/govmomi/session"
	"github.com/zhengkes/govmomi/simulator"
	"github.com/zhengkes/govmomi/sts"
	_ "github.com/zhengkes/govmomi/sts/simulator"
	"github.com/zhengkes/govmomi/vim25"
	"github.com/zhengkes/govmomi/vim25/soap"
)

func ExampleManager_LoginByToken() {
	simulator.Run(func(ctx context.Context, vc *vim25.Client) error {
		c, err := sts.NewClient(ctx, vc)
		if err != nil {
			return err
		}

		// Issue a bearer token
		req := sts.TokenRequest{
			Userinfo: url.UserPassword("Administrator@VSPHERE.LOCAL", "password"),
		}

		signer, err := c.Issue(ctx, req)
		if err != nil {
			return err
		}

		// Create a new un-authenticated client and LoginByToken
		vc2, err := vim25.NewClient(ctx, soap.NewClient(vc.URL(), true))
		if err != nil {
			return err
		}

		m := session.NewManager(vc2)
		header := soap.Header{Security: signer}

		err = m.LoginByToken(vc2.WithHeader(ctx, header))
		if err != nil {
			return err
		}

		session, err := m.UserSession(ctx)
		if err != nil {
			return err
		}

		fmt.Println(session.UserName)

		return nil
	})
	// Output: Administrator@VSPHERE.LOCAL
}
