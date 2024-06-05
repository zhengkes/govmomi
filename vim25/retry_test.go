/*
Copyright (c) 2015 VMware, Inc. All Rights Reserved.

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

package vim25_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/zhengkes/govmomi/find"
	"github.com/zhengkes/govmomi/simulator"
	"github.com/zhengkes/govmomi/vim25"
	"github.com/zhengkes/govmomi/vim25/soap"
	"github.com/zhengkes/govmomi/vim25/types"
)

type tempError struct{}

func (tempError) Error() string   { return "tempError" }
func (tempError) Timeout() bool   { return true }
func (tempError) Temporary() bool { return true }

type nonTempError struct{}

func (nonTempError) Error() string   { return "nonTempError" }
func (nonTempError) Timeout() bool   { return false }
func (nonTempError) Temporary() bool { return false }

type fakeRoundTripper struct {
	errs []error
}

func (f *fakeRoundTripper) RoundTrip(ctx context.Context, req, res soap.HasFault) error {
	err := f.errs[0]
	f.errs = f.errs[1:]
	return err
}

func TestRetry(t *testing.T) {
	var tcs = []struct {
		errs     []error
		expected error
	}{
		{
			errs:     []error{nil},
			expected: nil,
		},
		{
			errs:     []error{tempError{}, nil},
			expected: nil,
		},
		{
			errs:     []error{tempError{}, tempError{}},
			expected: tempError{},
		},
		{
			errs:     []error{nonTempError{}},
			expected: nonTempError{},
		},
		{
			errs:     []error{tempError{}, nonTempError{}},
			expected: nonTempError{},
		},
	}

	for _, tc := range tcs {
		var rt soap.RoundTripper = &fakeRoundTripper{errs: tc.errs}
		rt = vim25.Retry(rt, vim25.RetryTemporaryNetworkError, 2)

		err := rt.RoundTrip(context.TODO(), nil, nil)
		if err != tc.expected {
			t.Errorf("Expected: %s, got: %s", tc.expected, err)
		}
	}
}

func TestRetryNetworkError(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		c.RoundTripper = vim25.Retry(c.Client, vim25.RetryTemporaryNetworkError, 2)

		vm, err := find.NewFinder(c).VirtualMachine(ctx, "DC0_H0_VM0")
		if err != nil {
			t.Fatal(err)
		}

		// Tell vcsim to respond with 502 on the 1st request
		simulator.StatusSDK = http.StatusBadGateway

		state, err := vm.PowerState(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if state != types.VirtualMachinePowerStatePoweredOn {
			t.Errorf("state=%s", state)
		}

		retry := func(err error) (bool, time.Duration) {
			simulator.StatusSDK = http.StatusBadGateway // Tell vcsim to respond with 502 on every request
			return vim25.IsTemporaryNetworkError(err), 0
		}
		c.RoundTripper = vim25.Retry(c.Client, retry, 2)

		simulator.StatusSDK = http.StatusBadGateway
		// beyond max retry attempts, should result in an erro
		for i := 0; i < 3; i++ {
			_, err = vm.PowerState(ctx)
		}

		if err == nil {
			t.Error("expected error")
		}

		if !vim25.IsTemporaryNetworkError(err) {
			t.Errorf("unexpected error=%s", err)
		}
		simulator.StatusSDK = http.StatusOK
	})
}
