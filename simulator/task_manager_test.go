/*
Copyright (c) 2017-2024 VMware, Inc. All Rights Reserved.

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

package simulator_test

import (
	"context"
	"testing"

	"github.com/zhengkes/govmomi/object"
	"github.com/zhengkes/govmomi/property"
	"github.com/zhengkes/govmomi/simulator"
	"github.com/zhengkes/govmomi/task"
	"github.com/zhengkes/govmomi/vim25"
	"github.com/zhengkes/govmomi/vim25/mo"
	"github.com/zhengkes/govmomi/vim25/types"
)

func TestTaskManagerRecent(t *testing.T) {
	simulator.Test(func(ctx context.Context, c *vim25.Client) {
		ref := simulator.Map.Any("VirtualMachine").Reference()
		vm := object.NewVirtualMachine(c, ref)

		tasks := func() int {
			var m mo.TaskManager
			pc := property.DefaultCollector(c)
			err := pc.RetrieveOne(ctx, *c.ServiceContent.TaskManager, nil, &m)
			if err != nil {
				t.Fatal(err)
			}
			return len(m.RecentTask)
		}

		start := tasks()
		if start == 0 {
			t.Fatal("recentTask is empty")
		}

		task, err := vm.PowerOff(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err = task.WaitEx(ctx); err != nil {
			t.Fatal(err)
		}

		end := tasks()
		if end == 0 {
			t.Fatal("recentTask is empty")
		}
		if start == end {
			t.Fatal("recentTask not updated")
		}
	})
}

func TestTaskManagerRead(t *testing.T) {
	simulator.Test(func(ctx context.Context, vc *vim25.Client) {
		spec := types.TaskFilterSpec{
			Entity: &types.TaskFilterSpecByEntity{
				Entity:    vc.ServiceContent.RootFolder,
				Recursion: types.TaskFilterSpecRecursionOptionAll,
			},
		}
		tm := task.NewManager(vc)
		c, err := tm.CreateCollectorForTasks(ctx, spec)
		if err != nil {
			t.Fatal(err)
		}

		page, err := c.LatestPage(ctx)
		if err != nil {
			t.Fatal(err)
		}
		ntasks := len(page)
		if ntasks == 0 {
			t.Fatal("no recent tasks")
		}
		tests := []struct {
			max    int
			rewind bool
			order  bool
			read   func(context.Context, int32) ([]types.TaskInfo, error)
		}{
			{ntasks, true, true, c.ReadNextTasks},
			{ntasks / 3, true, true, c.ReadNextTasks},
			{ntasks * 3, false, true, c.ReadNextTasks},
			{3, false, false, c.ReadPreviousTasks},
			{ntasks * 3, false, true, c.ReadNextTasks},
		}

		for _, test := range tests {
			var all []types.TaskInfo
			count := 0
			for {
				tasks, err := test.read(ctx, int32(test.max))
				if err != nil {
					t.Fatal(err)
				}
				if len(tasks) == 0 {
					// expecting 0 below as we've read all tasks in the page
					ztasks, nerr := test.read(ctx, int32(test.max))
					if nerr != nil {
						t.Fatal(nerr)
					}
					if len(ztasks) != 0 {
						t.Errorf("ztasks=%d", len(ztasks))
					}
					break
				}
				count += len(tasks)
				all = append(all, tasks...)
			}
			if count < len(page) {
				t.Errorf("expected at least %d tasks, got: %d", len(page), count)
			}

			if test.rewind {
				if err = c.Rewind(ctx); err != nil {
					t.Error(err)
				}
			}
		}

		// after Reset() we should only get tasks via ReadPreviousTasks
		if err = c.Reset(ctx); err != nil {
			t.Fatal(err)
		}

		tasks, err := c.ReadNextTasks(ctx, int32(ntasks))
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 0 {
			t.Errorf("expected 0 tasks, got %d", len(tasks))
		}

		ref := simulator.Map.Any("VirtualMachine").Reference()
		vm := object.NewVirtualMachine(vc, ref)
		if _, err = vm.PowerOff(ctx); err != nil {
			t.Fatal(err)
		}

		tasks, err = c.ReadNextTasks(ctx, int32(ntasks))
		if err != nil {
			t.Fatal(err)
		}
		if len(tasks) != 1 {
			t.Errorf("expected 1 tasks, got %d", len(tasks))
		}

		count := 0
		for {
			tasks, err = c.ReadPreviousTasks(ctx, 3)
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) == 0 {
				break
			}
			count += len(tasks)
		}
		if count < ntasks {
			t.Errorf("expected %d tasks, got %d", ntasks, count)
		}
	})
}
