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

package cluster

import (
	"context"
	"flag"
	"fmt"

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/object"
)

type mv struct {
	*flags.ClusterFlag
}

func init() {
	cli.Register("cluster.mv", &mv{})
}

func (cmd *mv) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClusterFlag, ctx = flags.NewClusterFlag(ctx)
	cmd.ClusterFlag.Register(ctx, f)
}

func (cmd *mv) Process(ctx context.Context) error {
	if err := cmd.ClusterFlag.Process(ctx); err != nil {
		return err
	}

	return nil
}

func (cmd *mv) Description() string {
	return `Move HOST to CLUSTER.

The hosts are moved to the cluster specified by the 'cluster' flag.

Examples:
  govc cluster.mv -cluster ClusterA host1 host2`
}

func (cmd *mv) Move(ctx context.Context, cluster *object.ClusterComputeResource, hosts []*object.HostSystem) error {
	task, err := cluster.MoveInto(ctx, hosts...)
	if err != nil {
		return err
	}

	logger := cmd.ProgressLogger(fmt.Sprintf("moving %d hosts to cluster %s... ", len(hosts), cluster.InventoryPath))
	defer logger.Wait()

	_, err = task.WaitForResult(ctx, logger)
	return err
}

func (cmd *mv) Run(ctx context.Context, f *flag.FlagSet) error {
	if f.NArg() == 0 {
		return flag.ErrHelp
	}

	cluster, err := cmd.Cluster()
	if err != nil {
		return err
	}

	finder, err := cmd.Finder()
	if err != nil {
		return err
	}

	var hosts []*object.HostSystem

	for _, path := range f.Args() {
		list, err := finder.HostSystemList(ctx, path)
		if err != nil {
			return err
		}
		hosts = append(hosts, list...)
	}

	return cmd.Move(ctx, cluster, hosts)
}
