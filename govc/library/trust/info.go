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

package trust

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"flag"
	"io"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/object"
	"github.com/zhengkes/govmomi/vapi/library"
)

type info struct {
	*flags.ClientFlag
	*flags.OutputFlag
}

func init() {
	cli.Register("library.trust.info", &info{})
}

func (cmd *info) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)
	cmd.OutputFlag.Register(ctx, f)
}

func (cmd *info) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return nil
}

func (cmd *info) Usage() string {
	return "ID"
}

func (cmd *info) Description() string {
	return `Display trusted certificate info.

Examples:
  govc library.trust.info vmware_signed`
}

type infoResultsWriter struct {
	TrustedCertificateInfo *library.TrustedCertificate `json:"info,omitempty"`
}

func (r infoResultsWriter) Write(w io.Writer) error {
	block, _ := pem.Decode([]byte(r.TrustedCertificateInfo.Text))
	x, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	var info object.HostCertificateInfo
	info.FromCertificate(x)

	return info.Write(w)
}

func (r infoResultsWriter) Dump() interface{} {
	return r.TrustedCertificateInfo
}

func (cmd *info) Run(ctx context.Context, f *flag.FlagSet) error {
	if f.NArg() != 1 {
		return flag.ErrHelp
	}

	c, err := cmd.RestClient()
	if err != nil {
		return err
	}

	cert, err := library.NewManager(c).GetTrustedCertificate(ctx, f.Arg(0))
	if err != nil {
		return err
	}
	return cmd.WriteResult(&infoResultsWriter{cert})
}
