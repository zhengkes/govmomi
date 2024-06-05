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

package library

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/vapi/library"
)

type probe struct {
	*flags.ClientFlag
	*flags.OutputFlag

	fail bool
}

func init() {
	cli.Register("library.probe", &probe{}, true)
}

func (cmd *probe) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.OutputFlag.Register(ctx, f)

	f.BoolVar(&cmd.fail, "f", false, "Fail if probe status is not success")
}

func (cmd *probe) Usage() string {
	return "URI"
}

func (cmd *probe) Description() string {
	return `Probes the source endpoint URI with https or http schemes.

Examples:
  govc library.probe https://example.com/file.ova`
}

type probeResult struct {
	*library.ProbeResult
}

func (r *probeResult) Write(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 2, 0, 2, ' ', 0)

	fmt.Fprintf(tw, "Status:\t%s\n", r.Status)
	thumbprint := r.SSLThumbprint
	if thumbprint == "" {
		thumbprint = "-"
	}
	fmt.Fprintf(tw, "Thumbprint:\t%s\n", thumbprint)
	for _, e := range r.ErrorMessages {
		fmt.Fprintf(tw, "%s:\t%s\n", e.ID, e.Error())
	}

	return tw.Flush()
}

func (cmd *probe) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return cmd.OutputFlag.Process(ctx)
}

func (cmd *probe) Run(ctx context.Context, f *flag.FlagSet) error {
	if f.NArg() != 1 {
		return flag.ErrHelp
	}

	c, err := cmd.RestClient()
	if err != nil {
		return err
	}

	m := library.NewManager(c)

	p, err := m.ProbeTransferEndpoint(ctx, library.TransferEndpoint{URI: f.Arg(0)})
	if err != nil {
		return err
	}

	if cmd.fail && p.Status != "SUCCESS" {
		cmd.Out = os.Stderr
		// using same exit code as curl -f:
		defer os.Exit(22)
	}

	return cmd.WriteResult(&probeResult{p})
}
