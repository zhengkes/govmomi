/*
Copyright (c) 2017 VMware, Inc. All Rights Reserved.

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

package vm

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/object"
	"github.com/zhengkes/govmomi/session"
	"github.com/zhengkes/govmomi/vim25/soap"
	"github.com/zhengkes/govmomi/vim25/types"
)

type console struct {
	*flags.VirtualMachineFlag
	//1
	*flags.OutputFlag
	h5      bool
	wss     bool
	capture string
	link    string
}

func init() {
	cli.Register("vm.console", &console{})
}

func (cmd *console) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.VirtualMachineFlag, ctx = flags.NewVirtualMachineFlag(ctx)
	cmd.VirtualMachineFlag.Register(ctx, f)
	//2
	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.OutputFlag.Register(ctx, f)

	f.BoolVar(&cmd.h5, "h5", false, "Generate HTML5 UI console link")
	f.BoolVar(&cmd.wss, "wss", false, "Generate WebSocket console link")
	f.StringVar(&cmd.capture, "capture", "", "Capture console screen shot to file")
}

func (cmd *console) Process(ctx context.Context) error {
	if err := cmd.VirtualMachineFlag.Process(ctx); err != nil {
		return err
	}
	return nil
}

func (cmd *console) Usage() string {
	return "VM"
}

func (cmd *console) Description() string {
	return `Generate console URL or screen capture for VM.

One of VMRC, VMware Player, VMware Fusion or VMware Workstation must be installed to
open VMRC console URLs.

Examples:
  govc vm.console my-vm
  govc vm.console -capture screen.png my-vm  # screen capture
  govc vm.console -capture - my-vm | display # screen capture to stdout
  open $(govc vm.console my-vm)              # MacOSX VMRC
  open $(govc vm.console -h5 my-vm)          # MacOSX H5
  xdg-open $(govc vm.console my-vm)          # Linux VMRC
  xdg-open $(govc vm.console -h5 my-vm)      # Linux H5`
}

func (cmd *console) Run(ctx context.Context, f *flag.FlagSet) error {
	vms, err := cmd.VirtualMachines(f.Args())
	if err != nil {
		return err
	}

	if len(vms) != 1 {
		return flag.ErrHelp
	}

	vm := vms[0]

	state, err := vm.PowerState(ctx)
	if err != nil {
		return err
	}

	if (cmd.capture != "" || cmd.wss) && state != types.VirtualMachinePowerStatePoweredOn {
		return fmt.Errorf("vm is not powered on (%s)", state)
	}

	c := vm.Client()

	u := c.URL()

	if cmd.capture != "" {
		u.Path = "/screen"
		query := url.Values{"id": []string{vm.Reference().Value}}
		u.RawQuery = query.Encode()

		param := soap.DefaultDownload

		if cmd.capture == "-" {
			w, _, derr := c.Download(ctx, u, &param)
			if derr != nil {
				return derr
			}

			_, err = io.Copy(os.Stdout, w)
			if err != nil {
				return err
			}

			return w.Close()
		}

		return c.DownloadFile(ctx, cmd.capture, u, &param)
	}

	if cmd.wss {
		ticket, err := vm.AcquireTicket(ctx, string(types.VirtualMachineTicketTypeWebmks))
		if err != nil {
			return err
		}

		link := fmt.Sprintf("wss://%s:%d/ticket/%s", ticket.Host, ticket.Port, ticket.Ticket)
		cmd.link = link
		return nil
	}

	m := session.NewManager(c)
	ticket, err := m.AcquireCloneTicket(ctx)
	if err != nil {
		return err
	}

	if cmd.h5 {
		m := object.NewOptionManager(c, *c.ServiceContent.Setting)

		opt, err := m.Query(ctx, "VirtualCenter.FQDN")
		if err != nil {
			return err
		}

		fqdn := opt[0].GetOptionValue().Value.(string)

		var info object.HostCertificateInfo
		err = info.FromURL(u, nil)
		if err != nil {
			return err
		}

		u.Path = "/ui/webconsole.html"

		u.RawQuery = url.Values{
			"vmId":          []string{vm.Reference().Value},
			"vmName":        []string{vm.Name()},
			"serverGuid":    []string{c.ServiceContent.About.InstanceUuid},
			"host":          []string{fqdn},
			"sessionTicket": []string{ticket},
			"thumbprint":    []string{info.ThumbprintSHA1},
		}.Encode()

		cmd.link = u.String()
	} else {
		cmd.link = fmt.Sprintf("vmrc://clone:%s@%s/?moid=%s", ticket, u.Hostname(), vm.Reference().Value)
	}
	return nil
}

// 3
func (cmd *console) Dump() interface{} {
	return cmd.link
}
