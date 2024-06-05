/*
Copyright (c) 2024-2024 VMware, Inc. All Rights Reserved.

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

	"github.com/zhengkes/govmomi/govc/cli"
	"github.com/zhengkes/govmomi/govc/flags"
	"github.com/zhengkes/govmomi/vapi/library"
)

type evict struct {
	*flags.ClientFlag
}

func init() {
	cli.Register("library.evict", &evict{})
}

func (cmd *evict) Register(ctx context.Context, f *flag.FlagSet) {
	cmd.ClientFlag, ctx = flags.NewClientFlag(ctx)
	cmd.ClientFlag.Register(ctx, f)
}

func (cmd *evict) Usage() string {
	return "LIBRARY NAME | ITEM NAME"
}

func (cmd *evict) Description() string {
	return `Evict library NAME or item NAME.

Examples:
  govc library.evict subscribed-library
  govc library.evict subscribed-library/item`
}

func (cmd *evict) Run(ctx context.Context, f *flag.FlagSet) error {
	if f.NArg() != 1 {
		return flag.ErrHelp
	}

	c, err := cmd.RestClient()
	if err != nil {
		return err
	}

	m := library.NewManager(c)

	res, err := flags.ContentLibraryResult(ctx, c, "", f.Arg(0))
	if err != nil {
		return err
	}

	switch t := res.GetResult().(type) {
	case library.Library:
		return m.EvictSubscribedLibrary(ctx, &t)
	case library.Item:
		return m.EvictSubscribedLibraryItem(ctx, &t)
	default:
		return fmt.Errorf("%q is a %T", f.Arg(0), t)
	}
}
