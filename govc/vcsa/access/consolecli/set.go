/*
Copyright (c) 2022 VMware, Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0.
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package consolecli

import (
	"context"
	"flag"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/vapi/appliance/access/consolecli"
)

type set struct {
	*flags.ClientFlag
	enabled bool
}

func init() {
	cli.Register("vcsa.access.consolecli.set", &set{})
}

func (cmd *set) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	f.BoolVar(&cmd.enabled,
		"enabled",
		false,
		"Enable Console CLI.")
}

func (cmd *set) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}

	return nil
}

func (cmd *set) Description() string {
	return `Set enabled state of the console-based controlled CLI (TTY1).

Note: This command requires vCenter 7.0.2 or higher.

Examples:
# Enable Console CLI
govc vcsa.access.consolecli.set -enabled=true

# Disable Console CLI
govc vcsa.access.consolecli.set -enabled=false`
}

func (cmd *set) Run(ctx context.Context, f *flag.FlagSet) error {
	c, err := cmd.RestClient()
	if err != nil {
		return err
	}

	m := consolecli.NewManager(c)

	input := consolecli.Access{Enabled: cmd.enabled}
	if err = m.Set(ctx, input); err != nil {
		return err
	}

	return nil
}
