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

package snapshot

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/zhengkes/govmomi/cns/types"
	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
)

type rm struct {
	*flags.ClientFlag
	*flags.OutputFlag
}

func init() {
	cli.Register("volume.snapshot.rm", &rm{})
}

func (cmd *rm) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)

	cmd.OutputFlag, ctx = flags.NewOutputFlag(ctx)
	cmd.OutputFlag.Register(ctx, f)
}

func (cmd *rm) Process(ctx context.Context) error {
	if err := cmd.ClientFlag.Process(ctx); err != nil {
		return err
	}
	return cmd.OutputFlag.Process(ctx)
}

func (cmd *rm) Usage() string {
	return "[SNAP_ID VOL_ID]..."
}

func (cmd *rm) Description() string {
	return `Remove snapshot SNAP_ID from volume VOL_ID.

Use a list of [SNAP_ID VOL_ID] pairs to remove multiple snapshots at once.

Examples:
  govc volume.snapshot.rm f75989dc-95b9-4db7-af96-8583f24bc59d df86393b-5ae0-4fca-87d0-b692dbc67d45
  govc volume.snapshot.rm $(govc volume.snapshot.ls -i df86393b-5ae0-4fca-87d0-b692dbc67d45)
  govc volume.snapshot.rm $(govc volume.snapshot.create -i df86393b-5ae0-4fca-87d0-b692dbc67d45 my-snapshot)
  govc volume.snapshot.rm $(govc volume.snapshot.ls -i $(govc volume.ls -i))`
}

type rmResult struct {
	VolumeResults []*types.CnsSnapshotDeleteResult `json:"volumeResults"`
	cmd           *rm
}

func (r *rmResult) Write(w io.Writer) error {
	var err error = nil
	tw := tabwriter.NewWriter(r.cmd.Out, 2, 0, 2, ' ', 0)
	for _, s := range r.VolumeResults {
		fmt.Fprintf(tw, "%s\t%s", s.SnapshotId.Id, s.VolumeId.Id)
		if s.Fault != nil {
			if err == nil {
				err = errors.New(s.Fault.LocalizedMessage)
			}
			fmt.Fprintf(tw, "\t%s", s.Fault.LocalizedMessage)
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
	return err
}

func (cmd *rm) Run(ctx context.Context, f *flag.FlagSet) error {
	if len(f.Args()) < 2 || len(f.Args())%2 != 0 {
		return flag.ErrHelp
	}

	c, err := cmd.CnsClient()
	if err != nil {
		return err
	}

	result := rmResult{cmd: cmd}

	for i := 0; i < len(f.Args()); i += 2 {
		spec := types.CnsSnapshotDeleteSpec{
			VolumeId: types.CnsVolumeId{
				Id: f.Arg(i + 1),
			},
			SnapshotId: types.CnsSnapshotId{
				Id: f.Arg(i),
			},
		}

		task, err := c.DeleteSnapshots(ctx, []types.CnsSnapshotDeleteSpec{spec})
		if err != nil {
			return err
		}

		info, err := task.WaitForResult(ctx, nil)
		if err != nil {
			return err
		}

		if res, ok := info.Result.(types.CnsVolumeOperationBatchResult); ok {
			if len(res.VolumeResults) > 0 {
				if sdr, ok := res.VolumeResults[0].(*types.CnsSnapshotDeleteResult); ok {
					if sdr.Fault != nil {
						if len(f.Args()) == 2 {
							return errors.New(sdr.Fault.LocalizedMessage)
						}
						sdr.SnapshotId.Id = f.Arg(i)
						sdr.VolumeId.Id = f.Arg(i + 1)
					}
					result.VolumeResults = append(result.VolumeResults, sdr)
				}
			}
		}
	}

	return cmd.WriteResult(&result)
}
