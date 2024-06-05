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

package simulator

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/zhengkes/govmomi"
	"github.com/zhengkes/govmomi/find"
	"github.com/zhengkes/govmomi/object"
	"github.com/zhengkes/govmomi/property"
	"github.com/zhengkes/govmomi/simulator/esx"
	"github.com/zhengkes/govmomi/task"
	"github.com/zhengkes/govmomi/vim25"
	"github.com/zhengkes/govmomi/vim25/mo"
	"github.com/zhengkes/govmomi/vim25/types"
)

func TestCreateVm(t *testing.T) {
	ctx := context.Background()

	for _, model := range []*Model{ESX(), VPX()} {
		defer model.Remove()
		err := model.Create()
		if err != nil {
			t.Fatal(err)
		}

		s := model.Service.NewServer()
		defer s.Close()

		c, err := govmomi.NewClient(ctx, s.URL, true)
		if err != nil {
			t.Fatal(err)
		}

		p := property.DefaultCollector(c.Client)

		finder := find.NewFinder(c.Client, false)

		dc, err := finder.DefaultDatacenter(ctx)
		if err != nil {
			t.Fatal(err)
		}

		finder.SetDatacenter(dc)

		folders, err := dc.Folders(ctx)
		if err != nil {
			t.Fatal(err)
		}

		ds, err := finder.DefaultDatastore(ctx)
		if err != nil {
			t.Fatal(err)
		}

		hosts, err := finder.HostSystemList(ctx, "*/*")
		if err != nil {
			t.Fatal(err)
		}

		nhosts := len(hosts)
		host := hosts[rand.Intn(nhosts)]
		pool, err := host.ResourcePool(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if nhosts == 1 {
			// test the default path against the ESX model
			host = nil
		}

		vmFolder := folders.VmFolder

		var vmx string

		spec := types.VirtualMachineConfigSpec{
			// Note: real ESX allows the VM to be created without a GuestId,
			// but will power on will fail.
			GuestId: string(types.VirtualMachineGuestOsIdentifierOtherGuest),
		}

		steps := []func(){
			func() {
				spec.Name = "test"
				vmx = fmt.Sprintf("%s/%s.vmx", spec.Name, spec.Name)
			},
			func() {
				spec.Files = &types.VirtualMachineFileInfo{
					VmPathName: fmt.Sprintf("[%s] %s", ds.Name(), vmx),
				}
			},
		}

		// expecting CreateVM to fail until all steps are taken
		for _, step := range steps {
			task, cerr := vmFolder.CreateVM(ctx, spec, pool, host)
			if cerr != nil {
				t.Fatal(err)
			}

			cerr = task.Wait(ctx)
			if cerr == nil {
				t.Error("expected error")
			}

			step()
		}

		task, err := vmFolder.CreateVM(ctx, spec, pool, host)
		if err != nil {
			t.Fatal(err)
		}

		info, err := task.WaitForResult(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}

		// Test that datastore files were created
		_, err = ds.Stat(ctx, vmx)
		if err != nil {
			t.Fatal(err)
		}

		vm := object.NewVirtualMachine(c.Client, info.Result.(types.ManagedObjectReference))

		name, err := vm.ObjectName(ctx)
		if err != nil {
			t.Fatal(err)
		}

		if name != spec.Name {
			t.Errorf("name=%s", name)
		}

		_, err = vm.Device(ctx)
		if err != nil {
			t.Fatal(err)
		}

		recreate := func(context.Context) (*object.Task, error) {
			return vmFolder.CreateVM(ctx, spec, pool, nil)
		}

		ops := []struct {
			method func(context.Context) (*object.Task, error)
			state  types.VirtualMachinePowerState
			fail   bool
		}{
			// Powered off by default
			{nil, types.VirtualMachinePowerStatePoweredOff, false},
			// Create with same .vmx path should fail
			{recreate, "", true},
			// Off -> On  == ok
			{vm.PowerOn, types.VirtualMachinePowerStatePoweredOn, false},
			// On  -> On  == fail
			{vm.PowerOn, types.VirtualMachinePowerStatePoweredOn, true},
			// On  -> Off == ok
			{vm.PowerOff, types.VirtualMachinePowerStatePoweredOff, false},
			// Off -> Off == fail
			{vm.PowerOff, types.VirtualMachinePowerStatePoweredOff, true},
			// Off -> On  == ok
			{vm.PowerOn, types.VirtualMachinePowerStatePoweredOn, false},
			// Destroy == fail (power is On)
			{vm.Destroy, types.VirtualMachinePowerStatePoweredOn, true},
			// On  -> Off == ok
			{vm.PowerOff, types.VirtualMachinePowerStatePoweredOff, false},
			// Off -> Reset == fail
			{vm.Reset, types.VirtualMachinePowerStatePoweredOff, true},
			// Off -> On  == ok
			{vm.PowerOn, types.VirtualMachinePowerStatePoweredOn, false},
			// On -> Reset == ok
			{vm.Reset, types.VirtualMachinePowerStatePoweredOn, false},
			// On -> Suspend == ok
			{vm.Suspend, types.VirtualMachinePowerStateSuspended, false},
			// On  -> Off == ok
			{vm.PowerOff, types.VirtualMachinePowerStatePoweredOff, false},
			// Destroy == ok (power is Off)
			{vm.Destroy, "", false},
		}

		for i, op := range ops {
			if op.method != nil {
				task, err = op.method(ctx)
				if err != nil {
					t.Fatal(err)
				}

				err = task.Wait(ctx)
				if op.fail {
					if err == nil {
						t.Errorf("%d: expected error", i)
					}
				} else {
					if err != nil {
						t.Errorf("%d: %s", i, err)
					}
				}
			}

			if len(op.state) != 0 {
				state, err := vm.PowerState(ctx)
				if err != nil {
					t.Fatal(err)
				}

				if state != op.state {
					t.Errorf("state=%s", state)
				}

				err = property.Wait(ctx, p, vm.Reference(), []string{object.PropRuntimePowerState}, func(pc []types.PropertyChange) bool {
					for _, c := range pc {
						switch v := c.Val.(type) {
						case types.VirtualMachinePowerState:
							if v != op.state {
								t.Errorf("state=%s", v)
							}
						default:
							t.Errorf("unexpected type %T", v)
						}

					}
					return true
				})
				if err != nil {
					t.Error(err)
				}

				running, err := vm.IsToolsRunning(ctx)
				if err != nil {
					t.Error(err)
				}
				if running {
					t.Error("tools running")
				}
			}
		}

		// Test that datastore files were removed
		_, err = ds.Stat(ctx, vmx)
		if err == nil {
			t.Error("expected error")
		}
	}
}

func TestCreateVmWithSpecialCharaters(t *testing.T) {
	tests := []struct {
		name     string
		expected string
	}{
		{`/`, `%2f`},
		{`\`, `%5c`},
		{`%`, `%25`},
		// multiple special characters
		{`%%`, `%25%25`},
		// slash-separated name
		{`foo/bar`, `foo%2fbar`},
	}

	for _, test := range tests {
		m := ESX()

		Test(func(ctx context.Context, c *vim25.Client) {
			finder := find.NewFinder(c, false)

			dc, err := finder.DefaultDatacenter(ctx)
			if err != nil {
				t.Fatal(err)
			}

			finder.SetDatacenter(dc)
			folders, err := dc.Folders(ctx)
			if err != nil {
				t.Fatal(err)
			}
			vmFolder := folders.VmFolder

			ds, err := finder.DefaultDatastore(ctx)
			if err != nil {
				t.Fatal(err)
			}

			spec := types.VirtualMachineConfigSpec{
				Name: test.name,
				Files: &types.VirtualMachineFileInfo{
					VmPathName: fmt.Sprintf("[%s]", ds.Name()),
				},
			}

			pool := object.NewResourcePool(c, esx.ResourcePool.Self)

			task, err := vmFolder.CreateVM(ctx, spec, pool, nil)
			if err != nil {
				t.Fatal(err)
			}

			info, err := task.WaitForResult(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}

			vm := object.NewVirtualMachine(c, info.Result.(types.ManagedObjectReference))
			name, err := vm.ObjectName(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if name != test.expected {
				t.Errorf("expected %s, got %s", test.expected, name)
			}
		}, m)
	}
}

func TestCloneVm(t *testing.T) {
	tests := []struct {
		name   string
		vmName string
		config types.VirtualMachineCloneSpec
		fail   bool
	}{
		{
			"clone a vm",
			"cloned-vm",
			types.VirtualMachineCloneSpec{
				Template: false,
				PowerOn:  false,
			},
			false,
		},
		{
			"vm name is duplicated",
			"DC0_H0_VM0",
			types.VirtualMachineCloneSpec{
				Template: false,
				PowerOn:  false,
			},
			true,
		},
	}

	for _, test := range tests {
		test := test // assign to local var since loop var is reused

		t.Run(test.name, func(t *testing.T) {
			m := VPX()
			defer m.Remove()

			Test(func(ctx context.Context, c *vim25.Client) {
				finder := find.NewFinder(c, false)
				dc, err := finder.DefaultDatacenter(ctx)
				if err != nil {
					t.Fatal(err)
				}

				folders, err := dc.Folders(ctx)
				if err != nil {
					t.Fatal(err)
				}

				vmFolder := folders.VmFolder

				vmm := Map.Any("VirtualMachine").(*VirtualMachine)
				vm := object.NewVirtualMachine(c, vmm.Reference())

				task, err := vm.Clone(ctx, vmFolder, test.vmName, test.config)
				if err != nil {
					t.Fatal(err)
				}

				err = task.Wait(ctx)
				if test.fail {
					if err == nil {
						t.Errorf("%s: expected error", test.name)
					}
				} else {
					if err != nil {
						t.Errorf("%s: %s", test.name, err)
					}
				}
			}, m)
		})
	}
}

func TestReconfigVmDevice(t *testing.T) {
	ctx := context.Background()

	m := ESX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	finder := find.NewFinder(c.Client, false)
	finder.SetDatacenter(object.NewDatacenter(c.Client, esx.Datacenter.Reference()))

	vms, err := finder.VirtualMachineList(ctx, "*")
	if err != nil {
		t.Fatal(err)
	}

	vm := vms[0]
	device, err := vm.Device(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// verify default device list
	_, err = device.FindIDEController("")
	if err != nil {
		t.Fatal(err)
	}

	// default list of devices + 1 NIC + 1 SCSI controller + 1 CDROM + 1 disk created by the Model
	mdevices := len(esx.VirtualDevice) + 4

	if len(device) != mdevices {
		t.Errorf("expected %d devices, got %d", mdevices, len(device))
	}

	d := device.FindByKey(esx.EthernetCard.Key)

	err = vm.AddDevice(ctx, d)
	if _, ok := err.(task.Error).Fault().(*types.InvalidDeviceSpec); !ok {
		t.Fatalf("err=%v", err)
	}

	err = vm.RemoveDevice(ctx, false, d)
	if err != nil {
		t.Fatal(err)
	}

	device, err = vm.Device(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(device) != mdevices-1 {
		t.Error("device list mismatch")
	}

	// cover the path where the simulator assigns a UnitNumber
	d.GetVirtualDevice().UnitNumber = nil
	// cover the path where the simulator assigns a Key
	d.GetVirtualDevice().Key = -1

	err = vm.AddDevice(ctx, d)
	if err != nil {
		t.Fatal(err)
	}

	device, err = vm.Device(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(device) != mdevices {
		t.Error("device list mismatch")
	}

	disks := device.SelectByType((*types.VirtualDisk)(nil))

	for _, d := range disks {
		disk := d.(*types.VirtualDisk)
		info := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)

		if info.Datastore.Type == "" || info.Datastore.Value == "" {
			t.Errorf("invalid datastore for %s", device.Name(d))
		}

		// RemoveDevice and keep the file backing
		if err = vm.RemoveDevice(ctx, true, d); err != nil {
			t.Error(err)
		}

		if err = vm.AddDevice(ctx, d); err == nil {
			t.Error("expected FileExists error")
		}

		// Need FileOperation=="" to add an existing disk, see object.VirtualMachine.configureDevice
		disk.CapacityInKB = 0
		disk.CapacityInBytes = 0
		if err = vm.AddDevice(ctx, d); err != nil {
			t.Error(err)
		}

		d.GetVirtualDevice().DeviceInfo = nil
		if err = vm.EditDevice(ctx, d); err != nil {
			t.Error(err)
		}

		// RemoveDevice and delete the file backing
		if err = vm.RemoveDevice(ctx, false, d); err != nil {
			t.Error(err)
		}

		if err = vm.AddDevice(ctx, d); err == nil {
			t.Error("expected FileNotFound error")
		}
	}
}

func TestConnectVmDevice(t *testing.T) {
	ctx := context.Background()

	m := ESX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	vmm := Map.Any("VirtualMachine").(*VirtualMachine)
	vm := object.NewVirtualMachine(c.Client, vmm.Reference())

	l := object.VirtualDeviceList{} // used only for Connect/Disconnect function

	tests := []struct {
		description            string
		changePower            func(context.Context) (*object.Task, error)
		changeConnectivity     func(types.BaseVirtualDevice) error
		expectedConnected      bool
		expectedStartConnected bool
	}{
		{"disconnect when vm is on", nil, l.Disconnect, false, false},
		{"connect when vm is on", nil, l.Connect, true, true},
		{"power off vm", vm.PowerOff, nil, false, true},
		{"disconnect when vm is off", nil, l.Disconnect, false, false},
		{"connect when vm is off", nil, l.Connect, false, true},
		{"power on vm when StartConnected is true", vm.PowerOn, nil, true, true},
		{"power off vm and disconnect again", vm.PowerOff, l.Disconnect, false, false},
		{"power on vm when StartConnected is false", vm.PowerOn, nil, false, false},
	}

	for _, testCase := range tests {
		testCase := testCase // assign to local var since loop var is reused

		t.Run(testCase.description, func(t *testing.T) {
			if testCase.changePower != nil {
				task, err := testCase.changePower(ctx)
				if err != nil {
					t.Fatal(err)
				}

				err = task.Wait(ctx)
				if err != nil {
					t.Fatal(err)
				}
			}

			if testCase.changeConnectivity != nil {
				list, err := vm.Device(ctx)
				if err != nil {
					t.Fatal(err)
				}
				device := list.FindByKey(esx.EthernetCard.Key)
				if device == nil {
					t.Fatal("cloud not find EthernetCard")
				}

				err = testCase.changeConnectivity(device)
				if err != nil {
					t.Fatal(err)
				}
				err = vm.EditDevice(ctx, device)
				if err != nil {
					t.Fatal(err)
				}
			}

			updatedList, err := vm.Device(ctx)
			if err != nil {
				t.Fatal(err)
			}
			updatedDevice := updatedList.FindByKey(esx.EthernetCard.Key)
			if updatedDevice == nil {
				t.Fatal("cloud not find EthernetCard")
			}
			conn := updatedDevice.GetVirtualDevice().Connectable

			if conn.Connected != testCase.expectedConnected {
				t.Errorf("unexpected Connected property. expected: %t, actual: %t",
					testCase.expectedConnected, conn.Connected)
			}
			if conn.StartConnected != testCase.expectedStartConnected {
				t.Errorf("unexpected StartConnected property. expected: %t, actual: %t",
					testCase.expectedStartConnected, conn.StartConnected)
			}
		})
	}
}

func TestVAppConfigAdd(t *testing.T) {
	ctx := context.Background()

	m := ESX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	vmm := Map.Any("VirtualMachine").(*VirtualMachine)
	vm := object.NewVirtualMachine(c.Client, vmm.Reference())

	tests := []struct {
		description      string
		expectedErr      types.BaseMethodFault
		spec             types.VirtualMachineConfigSpec
		existingVMConfig *types.VirtualMachineConfigSpec
		expectedProps    []types.VAppPropertyInfo
	}{

		{
			description: "successfully add a new property",
			spec: types.VirtualMachineConfigSpec{
				VAppConfig: &types.VmConfigSpec{
					Property: []types.VAppPropertySpec{
						{
							ArrayUpdateSpec: types.ArrayUpdateSpec{
								Operation: types.ArrayUpdateOperationAdd,
							},
							Info: &types.VAppPropertyInfo{
								Key:   int32(1),
								Id:    "foo-id",
								Value: "foo-value",
							},
						},
					},
				},
			},
			expectedProps: []types.VAppPropertyInfo{
				{
					Key:   int32(1),
					Id:    "foo-id",
					Value: "foo-value",
				},
			},
		},
		{
			description: "return error when a property that exists is added",
			expectedErr: new(types.InvalidArgument),
			existingVMConfig: &types.VirtualMachineConfigSpec{
				VAppConfig: &types.VmConfigSpec{
					Property: []types.VAppPropertySpec{
						{
							ArrayUpdateSpec: types.ArrayUpdateSpec{
								Operation: types.ArrayUpdateOperationAdd,
							},
							Info: &types.VAppPropertyInfo{
								Key:   int32(2),
								Id:    "foo-id",
								Value: "foo-value",
							},
						},
					},
				},
			},
			spec: types.VirtualMachineConfigSpec{
				VAppConfig: &types.VmConfigSpec{
					Property: []types.VAppPropertySpec{
						{
							ArrayUpdateSpec: types.ArrayUpdateSpec{
								Operation: types.ArrayUpdateOperationAdd,
							},
							Info: &types.VAppPropertyInfo{
								Key: int32(2),
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.description, func(t *testing.T) {
			if testCase.existingVMConfig != nil {
				rtask, _ := vm.Reconfigure(ctx, *testCase.existingVMConfig)
				if err := rtask.Wait(ctx); err != nil {
					t.Errorf("Reconfigure failed during test setup. err: %v", err)
				}
			}

			err := vmm.updateVAppProperty(testCase.spec.VAppConfig.GetVmConfigSpec())
			if !reflect.DeepEqual(err, testCase.expectedErr) {
				t.Errorf("unexpected error in updating VApp property of VM. expectedErr: %v, actualErr: %v", testCase.expectedErr, err)
			}

			if testCase.expectedErr == nil {
				props := vmm.Config.VAppConfig.GetVmConfigInfo().Property
				// the testcase only has one VApp property, so ordering of the elements does not matter.
				if !reflect.DeepEqual(props, testCase.expectedProps) {
					t.Errorf("unexpected VApp properties. expected: %v, actual: %v", testCase.expectedProps, props)
				}
			}
		})
	}
}

func TestVAppConfigEdit(t *testing.T) {
	ctx := context.Background()

	m := ESX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	vmm := Map.Any("VirtualMachine").(*VirtualMachine)
	vm := object.NewVirtualMachine(c.Client, vmm.Reference())

	tests := []struct {
		description      string
		expectedErr      types.BaseMethodFault
		spec             types.VirtualMachineConfigSpec
		existingVMConfig *types.VirtualMachineConfigSpec
		expectedProps    []types.VAppPropertyInfo
	}{

		{
			description: "successfully update a property that exists",
			existingVMConfig: &types.VirtualMachineConfigSpec{
				VAppConfig: &types.VmConfigSpec{
					Property: []types.VAppPropertySpec{
						{
							ArrayUpdateSpec: types.ArrayUpdateSpec{
								Operation: types.ArrayUpdateOperationAdd,
							},
							Info: &types.VAppPropertyInfo{
								Key:   int32(1),
								Id:    "foo-id",
								Value: "foo-value",
							},
						},
					},
				},
			},
			spec: types.VirtualMachineConfigSpec{
				VAppConfig: &types.VmConfigSpec{
					Property: []types.VAppPropertySpec{
						{
							ArrayUpdateSpec: types.ArrayUpdateSpec{
								Operation: types.ArrayUpdateOperationEdit,
							},
							Info: &types.VAppPropertyInfo{
								Key:   int32(1),
								Id:    "foo-id-updated",
								Value: "foo-value-updated",
							},
						},
					},
				},
			},
			expectedProps: []types.VAppPropertyInfo{
				{
					Key:   int32(1),
					Id:    "foo-id-updated",
					Value: "foo-value-updated",
				},
			},
		},
		{
			description: "return error when a property that doesn't exist is updated",
			expectedErr: new(types.InvalidArgument),
			spec: types.VirtualMachineConfigSpec{
				VAppConfig: &types.VmConfigSpec{
					Property: []types.VAppPropertySpec{
						{
							ArrayUpdateSpec: types.ArrayUpdateSpec{
								Operation: types.ArrayUpdateOperationEdit,
							},
							Info: &types.VAppPropertyInfo{
								Key: int32(2),
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.description, func(t *testing.T) {
			if testCase.existingVMConfig != nil {
				rtask, _ := vm.Reconfigure(ctx, *testCase.existingVMConfig)
				if err := rtask.Wait(ctx); err != nil {
					t.Errorf("Reconfigure failed during test setup. err: %v", err)
				}
			}

			err := vmm.updateVAppProperty(testCase.spec.VAppConfig.GetVmConfigSpec())
			if !reflect.DeepEqual(err, testCase.expectedErr) {
				t.Errorf("unexpected error in updating VApp property of VM. expectedErr: %v, actualErr: %v", testCase.expectedErr, err)
			}

			if testCase.expectedErr == nil {
				props := vmm.Config.VAppConfig.GetVmConfigInfo().Property
				// the testcase only has one VApp property, so ordering of the elements does not matter.
				if !reflect.DeepEqual(props, testCase.expectedProps) {
					t.Errorf("unexpected VApp properties. expected: %v, actual: %v", testCase.expectedProps, props)
				}
			}
		})
	}
}

func TestVAppConfigRemove(t *testing.T) {
	ctx := context.Background()

	m := ESX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	vmm := Map.Any("VirtualMachine").(*VirtualMachine)
	vm := object.NewVirtualMachine(c.Client, vmm.Reference())

	tests := []struct {
		description      string
		expectedErr      types.BaseMethodFault
		spec             types.VirtualMachineConfigSpec
		existingVMConfig *types.VirtualMachineConfigSpec
		expectedProps    []types.VAppPropertyInfo
	}{
		{
			description: "returns success when a property that exists is removed",
			existingVMConfig: &types.VirtualMachineConfigSpec{
				VAppConfig: &types.VmConfigSpec{
					Property: []types.VAppPropertySpec{
						{
							ArrayUpdateSpec: types.ArrayUpdateSpec{
								Operation: types.ArrayUpdateOperationAdd,
							},
							Info: &types.VAppPropertyInfo{
								Key: int32(1),
							},
						},
					},
				},
			},
			spec: types.VirtualMachineConfigSpec{
				VAppConfig: &types.VmConfigSpec{
					Property: []types.VAppPropertySpec{
						{
							ArrayUpdateSpec: types.ArrayUpdateSpec{
								Operation: types.ArrayUpdateOperationRemove,
							},
							Info: &types.VAppPropertyInfo{
								Key: int32(1),
							},
						},
					},
				},
			},
			expectedProps: []types.VAppPropertyInfo{},
		},
		{
			description: "return error when a property that doesn't exist is removed",
			expectedErr: new(types.InvalidArgument),
			spec: types.VirtualMachineConfigSpec{
				VAppConfig: &types.VmConfigSpec{
					Property: []types.VAppPropertySpec{
						{
							ArrayUpdateSpec: types.ArrayUpdateSpec{
								Operation: types.ArrayUpdateOperationRemove,
							},
							Info: &types.VAppPropertyInfo{
								Key: int32(2),
							},
						},
					},
				},
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.description, func(t *testing.T) {
			if testCase.existingVMConfig != nil {
				rtask, _ := vm.Reconfigure(ctx, *testCase.existingVMConfig)
				if err := rtask.Wait(ctx); err != nil {
					t.Errorf("Reconfigure failed during test setup. err: %v", err)
				}
			}

			err := vmm.updateVAppProperty(testCase.spec.VAppConfig.GetVmConfigSpec())
			if !reflect.DeepEqual(err, testCase.expectedErr) {
				t.Errorf("unexpected error in updating VApp property of VM. expectedErr: %v, actualErr: %v", testCase.expectedErr, err)
			}

			if testCase.expectedErr == nil {
				props := vmm.Config.VAppConfig.GetVmConfigInfo().Property
				// the testcase only has one VApp property, so ordering of the elements does not matter.
				if !reflect.DeepEqual(props, testCase.expectedProps) {
					t.Errorf("unexpected VApp properties. expected: %v, actual: %v", testCase.expectedProps, props)
				}
			}
		})
	}
}

func TestReconfigVm(t *testing.T) {
	ctx := context.Background()

	m := ESX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	vmm := Map.Any("VirtualMachine").(*VirtualMachine)
	vm := object.NewVirtualMachine(c.Client, vmm.Reference())

	tests := []struct {
		fail bool
		spec types.VirtualMachineConfigSpec
	}{
		{
			true, types.VirtualMachineConfigSpec{
				CpuAllocation: &types.ResourceAllocationInfo{Reservation: types.NewInt64(-1)},
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				CpuAllocation: &types.ResourceAllocationInfo{Reservation: types.NewInt64(100)},
			},
		},
		{
			true, types.VirtualMachineConfigSpec{
				GuestId: "enoent",
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				GuestId: string(GuestID[0]),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				NestedHVEnabled: types.NewBool(true),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				CpuHotAddEnabled: types.NewBool(true),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				CpuHotRemoveEnabled: types.NewBool(true),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				GuestAutoLockEnabled: types.NewBool(true),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				MemoryHotAddEnabled: types.NewBool(true),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				MemoryReservationLockedToMax: types.NewBool(true),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				MessageBusTunnelEnabled: types.NewBool(true),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				NpivTemporaryDisabled: types.NewBool(true),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				NpivOnNonRdmDisks: types.NewBool(true),
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				ConsolePreferences: &types.VirtualMachineConsolePreferences{
					PowerOnWhenOpened: types.NewBool(true),
				},
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				CpuAffinity: &types.VirtualMachineAffinityInfo{
					AffinitySet: []int32{1},
				},
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				CpuAllocation: &types.ResourceAllocationInfo{
					Reservation: types.NewInt64(100),
				},
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				MemoryAffinity: &types.VirtualMachineAffinityInfo{
					AffinitySet: []int32{1},
				},
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				MemoryAllocation: &types.ResourceAllocationInfo{
					Reservation: types.NewInt64(100),
				},
			},
		},
		{
			false, types.VirtualMachineConfigSpec{
				LatencySensitivity: &types.LatencySensitivity{
					Sensitivity: 1,
				},
			},
		},
	}

	for i, test := range tests {
		rtask, _ := vm.Reconfigure(ctx, test.spec)

		err := rtask.Wait(ctx)
		if test.fail {
			if err == nil {
				t.Errorf("%d: expected failure", i)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected failure: %s", err)
			}
		}
	}

	// Verify ReConfig actually works
	if *vmm.Config.NestedHVEnabled != true {
		t.Errorf("vm.Config.NestedHVEnabled expected true; got false")
	}
	if *vmm.Config.CpuHotAddEnabled != true {
		t.Errorf("vm.Config.CpuHotAddEnabled expected true; got false")
	}
	if *vmm.Config.CpuHotRemoveEnabled != true {
		t.Errorf("vm.Config.CpuHotRemoveEnabled expected true; got false")
	}
	if *vmm.Config.GuestAutoLockEnabled != true {
		t.Errorf("vm.Config.GuestAutoLockEnabled expected true; got false")
	}
	if *vmm.Config.MemoryHotAddEnabled != true {
		t.Errorf("vm.Config.MemoryHotAddEnabled expected true; got false")
	}
	if *vmm.Config.MemoryReservationLockedToMax != true {
		t.Errorf("vm.Config.MemoryReservationLockedToMax expected true; got false")
	}
	if *vmm.Config.MessageBusTunnelEnabled != true {
		t.Errorf("vm.Config.MessageBusTunnelEnabled expected true; got false")
	}
	if *vmm.Config.NpivTemporaryDisabled != true {
		t.Errorf("vm.Config.NpivTemporaryDisabled expected true; got false")
	}
	if *vmm.Config.NpivOnNonRdmDisks != true {
		t.Errorf("vm.Config.NpivOnNonRdmDisks expected true; got false")
	}
	if *vmm.Config.ConsolePreferences.PowerOnWhenOpened != true {
		t.Errorf("vm.Config.ConsolePreferences.PowerOnWhenOpened expected true; got false")
	}
	if vmm.Config.CpuAffinity.AffinitySet[0] != int32(1) {
		t.Errorf("vm.Config.CpuAffinity.AffinitySet[0] expected %d; got %d",
			1, vmm.Config.CpuAffinity.AffinitySet[0])
	}
	if vmm.Config.MemoryAffinity.AffinitySet[0] != int32(1) {
		t.Errorf("vm.Config.CpuAffinity.AffinitySet[0] expected %d; got %d",
			1, vmm.Config.CpuAffinity.AffinitySet[0])
	}
	if *vmm.Config.CpuAllocation.Reservation != 100 {
		t.Errorf("vm.Config.CpuAllocation.Reservation expected %d; got %d",
			100, *vmm.Config.CpuAllocation.Reservation)
	}
	if *vmm.Config.MemoryAllocation.Reservation != 100 {
		t.Errorf("vm.Config.MemoryAllocation.Reservation expected %d; got %d",
			100, *vmm.Config.MemoryAllocation.Reservation)
	}
	if vmm.Config.LatencySensitivity.Sensitivity != int32(1) {
		t.Errorf("vmm.Config.LatencySensitivity.Sensitivity expected %d; got %d",
			1, vmm.Config.LatencySensitivity.Sensitivity)
	}
}

func TestCreateVmWithDevices(t *testing.T) {
	ctx := context.Background()

	m := ESX()
	m.Datastore = 2
	defer m.Remove()

	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c := m.Service.client

	folder := object.NewFolder(c, esx.Datacenter.VmFolder)
	pool := object.NewResourcePool(c, esx.ResourcePool.Self)

	// different set of devices from Model.Create's
	var devices object.VirtualDeviceList
	ide, _ := devices.CreateIDEController()
	cdrom, _ := devices.CreateCdrom(ide.(*types.VirtualIDEController))
	scsi, _ := devices.CreateSCSIController("scsi")
	disk := &types.VirtualDisk{
		CapacityInKB: 1024,
		VirtualDevice: types.VirtualDevice{
			Backing: new(types.VirtualDiskFlatVer2BackingInfo), // Leave fields empty to test defaults
		},
	}
	disk2 := &types.VirtualDisk{
		CapacityInKB: 1024,
		VirtualDevice: types.VirtualDevice{
			Backing: &types.VirtualDiskFlatVer2BackingInfo{
				VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
					FileName: "[LocalDS_0]",
				},
			},
		},
	}
	devices = append(devices, ide, cdrom, scsi)
	devices.AssignController(disk, scsi.(*types.VirtualLsiLogicController))
	devices = append(devices, disk)
	devices.AssignController(disk2, scsi.(*types.VirtualLsiLogicController))
	devices = append(devices, disk2)
	create, _ := devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)

	spec := types.VirtualMachineConfigSpec{
		Name:         "foo",
		GuestId:      string(types.VirtualMachineGuestOsIdentifierOtherGuest),
		DeviceChange: create,
		Files: &types.VirtualMachineFileInfo{
			VmPathName: "[LocalDS_0]",
		},
	}

	ctask, _ := folder.CreateVM(ctx, spec, pool, nil)
	info, err := ctask.WaitForResult(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	vm := Map.Get(info.Result.(types.ManagedObjectReference)).(*VirtualMachine)

	expect := len(esx.VirtualDevice) + len(devices)
	ndevice := len(vm.Config.Hardware.Device)

	if expect != ndevice {
		t.Errorf("expected %d, got %d", expect, ndevice)
	}

	// check number of disk and disk summary
	ndisk := 0
	for _, device := range vm.Config.Hardware.Device {
		disk, ok := device.(*types.VirtualDisk)
		if ok {
			ndisk++
			summary := disk.DeviceInfo.GetDescription().Summary
			if summary != "1,024 KB" {
				t.Errorf("expected '1,1024 KB', got %s", summary)
			}
		}
	}
	if ndisk != 2 {
		t.Errorf("expected 1 disk, got %d", ndisk)
	}

	// Add disk on another datastore with empty path (issue 1854)
	ovm := object.NewVirtualMachine(c, vm.Self)
	disk = &types.VirtualDisk{
		CapacityInKB: 1024,
		VirtualDevice: types.VirtualDevice{
			Backing: &types.VirtualDiskFlatVer2BackingInfo{
				VirtualDeviceFileBackingInfo: types.VirtualDeviceFileBackingInfo{
					FileName: "[LocalDS_1]",
				},
			},
		},
	}
	devices.AssignController(disk, scsi.(*types.VirtualLsiLogicController))
	devices = nil
	devices = append(devices, disk)
	create, _ = devices.ConfigSpec(types.VirtualDeviceConfigSpecOperationAdd)
	spec = types.VirtualMachineConfigSpec{DeviceChange: create}
	rtask, _ := ovm.Reconfigure(ctx, spec)
	_, err = rtask.WaitForResult(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddedDiskCapacity(t *testing.T) {
	tests := []struct {
		name                    string
		capacityInBytes         int64
		capacityInKB            int64
		expectedCapacityInBytes int64
		expectedCapacityInKB    int64
	}{
		{
			"specify capacityInBytes",
			512 * 1024,
			0,
			512 * 1024,
			512,
		},
		{
			"specify capacityInKB",
			0,
			512,
			512 * 1024,
			512,
		},
		{
			"specify both",
			512 * 1024,
			512,
			512 * 1024,
			512,
		},
		{
			"capacityInbytes takes precedence if two fields represents different capacity",
			512 * 1024,
			1024,
			512 * 1024,
			512,
		},
	}

	for _, test := range tests {
		test := test // assign to local var since loop var is reused
		t.Run(test.name, func(t *testing.T) {
			m := ESX()

			Test(func(ctx context.Context, c *vim25.Client) {
				vmm := Map.Any("VirtualMachine").(*VirtualMachine)
				vm := object.NewVirtualMachine(c, vmm.Reference())

				ds := Map.Any("Datastore").(*Datastore)

				devices, err := vm.Device(ctx)
				if err != nil {
					t.Fatal(err)
				}

				controller, err := devices.FindDiskController("")
				if err != nil {
					t.Fatal(err)
				}

				disk := devices.CreateDisk(controller, ds.Reference(), "")
				disk.CapacityInBytes = test.capacityInBytes
				disk.CapacityInKB = test.capacityInKB

				err = vm.AddDevice(ctx, disk)
				if err != nil {
					t.Fatal(err)
				}

				newDevices, err := vm.Device(ctx)
				if err != nil {
					t.Fatal(err)
				}
				disks := newDevices.SelectByType((*types.VirtualDisk)(nil))
				if len(disks) == 0 {
					t.Fatalf("len(disks)=%d", len(disks))
				}

				newDisk := disks[len(disks)-1].(*types.VirtualDisk)

				if newDisk.CapacityInBytes != test.expectedCapacityInBytes {
					t.Errorf("CapacityInBytes expected %d, got %d",
						test.expectedCapacityInBytes, newDisk.CapacityInBytes)
				}
				if newDisk.CapacityInKB != test.expectedCapacityInKB {
					t.Errorf("CapacityInKB expected %d, got %d",
						test.expectedCapacityInKB, newDisk.CapacityInKB)
				}

			}, m)
		})
	}
}

func TestEditedDiskCapacity(t *testing.T) {
	tests := []struct {
		name                    string
		capacityInBytes         int64
		capacityInKB            int64
		expectedCapacityInBytes int64
		expectedCapacityInKB    int64
		expectedErr             types.BaseMethodFault
	}{
		{
			"specify same capacities as before",
			10 * 1024 * 1024 * 1024, // 10GB
			10 * 1024 * 1024,        // 10GB
			10 * 1024 * 1024 * 1024, // 10GB
			10 * 1024 * 1024,        // 10GB
			nil,
		},
		{
			"increase only capacityInBytes",
			20 * 1024 * 1024 * 1024, // 20GB
			10 * 1024 * 1024,        // 10GB
			20 * 1024 * 1024 * 1024, // 20GB
			20 * 1024 * 1024,        // 20GB
			nil,
		},
		{
			"increase only capacityInKB",
			10 * 1024 * 1024 * 1024, // 10GB
			20 * 1024 * 1024,        // 20GB
			20 * 1024 * 1024 * 1024, // 20GB
			20 * 1024 * 1024,        // 20GB
			nil,
		},
		{
			"increase both capacityInBytes and capacityInKB",
			20 * 1024 * 1024 * 1024, // 20GB
			20 * 1024 * 1024,        // 20GB
			20 * 1024 * 1024 * 1024, // 20GB
			20 * 1024 * 1024,        // 20GB
			nil,
		},
		{
			"increase both capacityInBytes and capacityInKB but value is different",
			20 * 1024 * 1024 * 1024, // 20GB
			30 * 1024 * 1024,        // 30GB
			0,
			0,
			new(types.InvalidDeviceOperation),
		},
		{
			"decrease capacity",
			1 * 1024 * 1024 * 1024, // 1GB
			1 * 1024 * 1024,        // 1GB
			0,
			0,
			new(types.InvalidDeviceOperation),
		},
	}

	for _, test := range tests {
		test := test // assign to local var since loop var is reused
		t.Run(test.name, func(t *testing.T) {
			m := ESX()

			Test(func(ctx context.Context, c *vim25.Client) {
				vmm := Map.Any("VirtualMachine").(*VirtualMachine)
				vm := object.NewVirtualMachine(c, vmm.Reference())
				ds := Map.Any("Datastore").(*Datastore)

				// create a new 10GB disk
				devices, err := vm.Device(ctx)
				if err != nil {
					t.Fatal(err)
				}
				controller, err := devices.FindDiskController("")
				if err != nil {
					t.Fatal(err)
				}
				disk := devices.CreateDisk(controller, ds.Reference(), "")
				disk.CapacityInBytes = 10 * 1024 * 1024 * 1024 // 10GB
				err = vm.AddDevice(ctx, disk)
				if err != nil {
					t.Fatal(err)
				}

				// edit its capacity
				addedDevices, err := vm.Device(ctx)
				if err != nil {
					t.Fatal(err)
				}
				addedDisks := addedDevices.SelectByType((*types.VirtualDisk)(nil))
				if len(addedDisks) == 0 {
					t.Fatal("disk not found")
				}
				addedDisk := addedDisks[0].(*types.VirtualDisk)
				addedDisk.CapacityInBytes = test.capacityInBytes
				addedDisk.CapacityInKB = test.capacityInKB
				err = vm.EditDevice(ctx, addedDisk)

				if test.expectedErr != nil {
					terr, ok := err.(task.Error)
					if !ok {
						t.Fatalf("error should be task.Error. actual: %T", err)
					}

					if !reflect.DeepEqual(terr.Fault(), test.expectedErr) {
						t.Errorf("expectedErr: %v, actualErr: %v", test.expectedErr, terr.Fault())
					}
				} else {
					// obtain the disk again
					editedDevices, err := vm.Device(ctx)
					if err != nil {
						t.Fatal(err)
					}
					editedDisks := editedDevices.SelectByType((*types.VirtualDisk)(nil))
					if len(editedDevices) == 0 {
						t.Fatal("disk not found")
					}
					editedDisk := editedDisks[len(editedDisks)-1].(*types.VirtualDisk)

					if editedDisk.CapacityInBytes != test.expectedCapacityInBytes {
						t.Errorf("CapacityInBytes expected %d, got %d",
							test.expectedCapacityInBytes, editedDisk.CapacityInBytes)
					}
					if editedDisk.CapacityInKB != test.expectedCapacityInKB {
						t.Errorf("CapacityInKB expected %d, got %d",
							test.expectedCapacityInKB, editedDisk.CapacityInKB)
					}
				}
			}, m)
		})
	}
}

func TestReconfigureDevicesDatastoreFreespace(t *testing.T) {
	tests := []struct {
		name          string
		reconfigure   func(context.Context, *object.VirtualMachine, *Datastore, object.VirtualDeviceList) error
		freespaceDiff int64
	}{
		{
			"create a new disk",
			func(ctx context.Context, vm *object.VirtualMachine, ds *Datastore, l object.VirtualDeviceList) error {
				controller, err := l.FindDiskController("")
				if err != nil {
					return err
				}

				disk := l.CreateDisk(controller, ds.Reference(), "")
				disk.CapacityInBytes = 10 * 1024 * 1024 * 1024 // 10GB

				if err := vm.AddDevice(ctx, disk); err != nil {
					return err
				}
				return nil
			},
			-10 * 1024 * 1024 * 1024, // -10GB
		},
		{
			"edit disk size",
			func(ctx context.Context, vm *object.VirtualMachine, ds *Datastore, l object.VirtualDeviceList) error {
				disks := l.SelectByType((*types.VirtualDisk)(nil))
				if len(disks) == 0 {
					return fmt.Errorf("disk not found")
				}
				disk := disks[len(disks)-1].(*types.VirtualDisk)

				// specify same disk capacity
				if err := vm.EditDevice(ctx, disk); err != nil {
					return err
				}
				return nil
			},
			0,
		},
		{
			"remove a disk and its files",
			func(ctx context.Context, vm *object.VirtualMachine, ds *Datastore, l object.VirtualDeviceList) error {
				disks := l.SelectByType((*types.VirtualDisk)(nil))
				if len(disks) == 0 {
					return fmt.Errorf("disk not found")
				}
				disk := disks[len(disks)-1].(*types.VirtualDisk)

				if err := vm.RemoveDevice(ctx, false, disk); err != nil {
					return err
				}
				return nil
			},
			10 * 1024 * 1024 * 1024, // 10GB
		},
		{
			"remove a disk but keep its files",
			func(ctx context.Context, vm *object.VirtualMachine, ds *Datastore, l object.VirtualDeviceList) error {
				disks := l.SelectByType((*types.VirtualDisk)(nil))
				if len(disks) == 0 {
					return fmt.Errorf("disk not found")
				}
				disk := disks[len(disks)-1].(*types.VirtualDisk)

				if err := vm.RemoveDevice(ctx, true, disk); err != nil {
					return err
				}
				return nil
			},
			0,
		},
	}

	for _, test := range tests {
		test := test // assign to local var since loop var is reused
		t.Run(test.name, func(t *testing.T) {
			m := ESX()

			Test(func(ctx context.Context, c *vim25.Client) {
				vmm := Map.Any("VirtualMachine").(*VirtualMachine)
				vm := object.NewVirtualMachine(c, vmm.Reference())

				ds := Map.Any("Datastore").(*Datastore)
				freespaceBefore := ds.Datastore.Summary.FreeSpace

				devices, err := vm.Device(ctx)
				if err != nil {
					t.Fatal(err)
				}

				err = test.reconfigure(ctx, vm, ds, devices)
				if err != nil {
					t.Fatal(err)
				}

				freespaceAfter := ds.Datastore.Summary.FreeSpace

				if freespaceAfter-freespaceBefore != test.freespaceDiff {
					t.Errorf("difference of freespace expected %d, got %d",
						test.freespaceDiff, freespaceAfter-freespaceBefore)
				}
			}, m)
		})
	}
}

func TestShutdownGuest(t *testing.T) {
	Test(func(ctx context.Context, c *vim25.Client) {
		vm := object.NewVirtualMachine(c, Map.Any("VirtualMachine").Reference())

		for _, timeout := range []bool{false, true} {
			if timeout {
				// ShutdownGuest will return right away, but powerState
				// is not updated until the internal task completes
				TaskDelay.MethodDelay = map[string]int{
					"ShutdownGuest": 500, // delay 500ms
					"LockHandoff":   0,   // don't lock vm during the delay
				}
			}

			err := vm.ShutdownGuest(ctx)
			if err != nil {
				t.Fatal(err)
			}

			wait := ctx
			var cancel context.CancelFunc
			if timeout {
				state, err := vm.PowerState(ctx)
				if err != nil {
					t.Fatal(err)
				}

				// with the task delay, should still be on at this point
				if state != types.VirtualMachinePowerStatePoweredOn {
					t.Errorf("state=%s", state)
				}

				wait, cancel = context.WithTimeout(ctx, time.Millisecond*250) // wait < task delay
				defer cancel()
			}

			err = vm.WaitForPowerState(wait, types.VirtualMachinePowerStatePoweredOff)
			if timeout {
				if err == nil {
					t.Error("expected timeout")
				}
				// wait for power state to change, else next test may fail
				err = vm.WaitForPowerState(ctx, types.VirtualMachinePowerStatePoweredOff)
				if err != nil {
					t.Fatal(err)
				}

			} else {
				if err != nil {
					t.Fatal(err)
				}
			}

			// shutdown a poweroff vm should fail
			err = vm.ShutdownGuest(ctx)
			if err == nil {
				t.Error("expected error: InvalidPowerState")
			}

			task, err := vm.PowerOn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = task.Wait(ctx)
			if err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestVmSnapshot(t *testing.T) {
	ctx := context.Background()

	m := ESX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	simVm := Map.Any("VirtualMachine")
	vm := object.NewVirtualMachine(c.Client, simVm.Reference())

	_, err = fieldValue(reflect.ValueOf(simVm), "snapshot")
	if err != errEmptyField {
		t.Fatal("snapshot property should be 'nil' if there are no snapshots")
	}

	task, err := vm.CreateSnapshot(ctx, "root", "description", true, true)
	if err != nil {
		t.Fatal(err)
	}

	info, err := task.WaitForResult(ctx)
	if err != nil {
		t.Fatal(err)
	}

	snapRef, ok := info.Result.(types.ManagedObjectReference)
	if !ok {
		t.Fatal("expected ManagedObjectRefrence result for CreateSnapshot")
	}

	_, err = vm.FindSnapshot(ctx, snapRef.Value)
	if err != nil {
		t.Fatal(err, "snapshot should be found by result reference")
	}

	_, err = fieldValue(reflect.ValueOf(simVm), "snapshot")
	if err == errEmptyField {
		t.Fatal("snapshot property should not be 'nil' if there are snapshots")
	}
	// NOTE: fieldValue cannot be used for nil check
	if len(simVm.(*VirtualMachine).RootSnapshot) == 0 {
		t.Fatal("rootSnapshot property should have elements if there are snapshots")
	}

	task, err = vm.CreateSnapshot(ctx, "child", "description", true, true)
	if err != nil {
		t.Fatal(err)
	}

	err = task.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = vm.FindSnapshot(ctx, "child")
	if err != nil {
		t.Fatal(err)
	}

	task, err = vm.RevertToCurrentSnapshot(ctx, true)
	if err != nil {
		t.Fatal(err)
	}

	err = task.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}

	task, err = vm.RevertToSnapshot(ctx, "root", true)
	if err != nil {
		t.Fatal(err)
	}

	err = task.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}

	task, err = vm.RemoveSnapshot(ctx, "child", false, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = task.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fieldValue(reflect.ValueOf(simVm), "snapshot")
	if err == errEmptyField {
		t.Fatal("snapshot property should not be 'nil' if there are snapshots")
	}
	// NOTE: fieldValue cannot be used for nil check
	if len(simVm.(*VirtualMachine).RootSnapshot) == 0 {
		t.Fatal("rootSnapshot property should have elements if there are snapshots")
	}

	_, err = vm.FindSnapshot(ctx, "child")
	if err == nil {
		t.Fatal("child should be removed")
	}

	task, err = vm.RemoveAllSnapshot(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = task.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fieldValue(reflect.ValueOf(simVm), "snapshot")
	if err != errEmptyField {
		t.Fatal("snapshot property should be 'nil' if there are no snapshots")
	}
	// NOTE: fieldValue cannot be used for nil check
	if len(simVm.(*VirtualMachine).RootSnapshot) != 0 {
		t.Fatal("rootSnapshot property should not have elements if there are no snapshots")
	}

	_, err = vm.FindSnapshot(ctx, "root")
	if err == nil {
		t.Fatal("all snapshots should be removed")
	}
}

func TestVmMarkAsTemplate(t *testing.T) {
	ctx := context.Background()

	m := VPX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	vm := object.NewVirtualMachine(c.Client, Map.Any("VirtualMachine").Reference())

	err = vm.MarkAsTemplate(ctx)
	if err == nil {
		t.Fatal("cannot create template for a powered on vm")
	}

	task, err := vm.PowerOff(ctx)
	if err != nil {
		t.Fatal(err)
	}

	task.Wait(ctx)

	err = vm.MarkAsTemplate(ctx)
	if err != nil {
		t.Fatal(err)
	}

	_, err = vm.PowerOn(ctx)
	if err == nil {
		t.Fatal("cannot PowerOn a template")
	}
}

func TestVmRefreshStorageInfo(t *testing.T) {
	ctx := context.Background()

	m := ESX()
	defer m.Remove()
	err := m.Create()
	if err != nil {
		t.Fatal(err)
	}

	s := m.Service.NewServer()
	defer s.Close()

	c, err := govmomi.NewClient(ctx, s.URL, true)
	if err != nil {
		t.Fatal(err)
	}

	vmm := Map.Any("VirtualMachine").(*VirtualMachine)
	vm := object.NewVirtualMachine(c.Client, vmm.Reference())

	// take snapshot
	task, err := vm.CreateSnapshot(ctx, "root", "description", true, true)
	if err != nil {
		t.Fatal(err)
	}

	err = task.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := vm.FindSnapshot(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}

	// check vm.Layout.Snapshot
	found := false
	for _, snapLayout := range vmm.Layout.Snapshot {
		if snapLayout.Key == *snapshot {
			found = true
		}
	}

	if found == false {
		t.Fatal("could not find new snapshot in vm.Layout.Snapshot")
	}

	// check vm.LayoutEx.Snapshot
	found = false
	for _, snapLayoutEx := range vmm.LayoutEx.Snapshot {
		if snapLayoutEx.Key == *snapshot {
			found = true
		}
	}

	if found == false {
		t.Fatal("could not find new snapshot in vm.LayoutEx.Snapshot")
	}

	// remove snapshot
	task, err = vm.RemoveAllSnapshot(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}

	err = task.Wait(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(vmm.Layout.Snapshot) != 0 {
		t.Fatal("expected vm.Layout.Snapshot to be empty")
	}

	if len(vmm.LayoutEx.Snapshot) != 0 {
		t.Fatal("expected vm.LayoutEx.Snapshot to be empty")
	}

	device, err := vm.Device(ctx)
	if err != nil {
		t.Fatal(err)
	}

	disks := device.SelectByType((*types.VirtualDisk)(nil))
	if len(disks) < 1 {
		t.Fatal("expected VM to have at least 1 disk")
	}

	findDiskFile := func(vmdkName string) *types.VirtualMachineFileLayoutExFileInfo {
		for _, dFile := range vmm.LayoutEx.File {
			if dFile.Name == vmdkName {
				return &dFile
			}
		}

		return nil
	}

	findDsStorage := func(dsName string) *types.VirtualMachineUsageOnDatastore {
		host := Map.Get(*vmm.Runtime.Host).(*HostSystem)
		ds := Map.FindByName(dsName, host.Datastore).(*Datastore)

		for _, dsUsage := range vmm.Storage.PerDatastoreUsage {
			if dsUsage.Datastore == ds.Self {
				return &dsUsage
			}
		}

		return nil
	}

	for _, d := range disks {
		disk := d.(*types.VirtualDisk)
		info := disk.Backing.(*types.VirtualDiskFlatVer2BackingInfo)
		diskLayoutCount := len(vmm.Layout.Disk)
		summaryStorageNew := vmm.Summary.Storage

		p, fault := parseDatastorePath(info.FileName)
		if fault != nil {
			t.Fatalf("could not parse datastore path for disk file: %s", info.FileName)
		}

		storageNew := findDsStorage(p.Datastore)
		if storageNew == nil {
			t.Fatalf("could not find vm usage on datastore: %s", p.Datastore)
		}

		diskFile := findDiskFile(info.FileName)
		if diskFile == nil {
			t.Fatal("could not find disk file in vm.LayoutEx.File")
		}

		// remove disk
		if err = vm.RemoveDevice(ctx, false, d); err != nil {
			t.Error(err)
		}

		summaryStorageOld := summaryStorageNew
		summaryStorageNew = vmm.Summary.Storage

		storageOld := storageNew
		storageNew = findDsStorage(p.Datastore)
		if storageNew == nil {
			t.Fatalf("could not find vm usage on datastore: %s", p.Datastore)
		}

		tests := []struct {
			got      int64
			expected int64
		}{
			{int64(len(vmm.Layout.Disk)), int64(diskLayoutCount - 1)},
			{summaryStorageNew.Committed, summaryStorageOld.Committed - diskFile.Size},
			{summaryStorageNew.Unshared, summaryStorageOld.Unshared - diskFile.Size},
			{summaryStorageNew.Uncommitted, summaryStorageOld.Uncommitted - disk.CapacityInBytes + diskFile.Size},
			{storageNew.Committed, storageOld.Committed - diskFile.Size},
			{storageNew.Unshared, storageOld.Unshared - diskFile.Size},
			{storageNew.Uncommitted, storageOld.Uncommitted - disk.CapacityInBytes + diskFile.Size},
		}

		for _, test := range tests {
			if test.got != test.expected {
				t.Errorf("expected %d, got %d", test.expected, test.got)
			}
		}

		// add disk
		disk.CapacityInBytes = 1000000000
		if err = vm.AddDevice(ctx, d); err != nil {
			t.Error(err)
		}

		summaryStorageOld = summaryStorageNew
		summaryStorageNew = vmm.Summary.Storage

		storageOld = storageNew
		storageNew = findDsStorage(p.Datastore)
		if storageNew == nil {
			t.Fatalf("could not find vm usage on datastore: %s", p.Datastore)
		}

		diskFile = findDiskFile(info.FileName)
		if diskFile == nil {
			t.Fatal("could not find disk file in vm.LayoutEx.File")
		}

		tests = []struct {
			got      int64
			expected int64
		}{
			{int64(len(vmm.Layout.Disk)), int64(diskLayoutCount)},
			{summaryStorageNew.Committed, summaryStorageOld.Committed + diskFile.Size},
			{summaryStorageNew.Unshared, summaryStorageOld.Unshared + diskFile.Size},
			{summaryStorageNew.Uncommitted, summaryStorageOld.Uncommitted + disk.CapacityInBytes - diskFile.Size},
			{storageNew.Committed, storageOld.Committed + diskFile.Size},
			{storageNew.Unshared, storageOld.Unshared + diskFile.Size},
			{storageNew.Uncommitted, storageOld.Uncommitted + disk.CapacityInBytes - diskFile.Size},
		}

		for _, test := range tests {
			if test.got != test.expected {
				t.Errorf("expected %d, got %d", test.expected, test.got)
			}
		}
	}

	// manually create log file
	fileLayoutExCount := len(vmm.LayoutEx.File)

	p, fault := parseDatastorePath(vmm.Config.Files.LogDirectory)
	if fault != nil {
		t.Fatalf("could not parse datastore path: %s", vmm.Config.Files.LogDirectory)
	}

	f, fault := vmm.createFile(p.String(), "test.log", false)
	if fault != nil {
		t.Fatal("could not create log file")
	}

	if len(vmm.LayoutEx.File) != fileLayoutExCount {
		t.Errorf("expected %d, got %d", fileLayoutExCount, len(vmm.LayoutEx.File))
	}

	if err = vm.RefreshStorageInfo(ctx); err != nil {
		t.Error(err)
	}

	if len(vmm.LayoutEx.File) != fileLayoutExCount+1 {
		t.Errorf("expected %d, got %d", fileLayoutExCount+1, len(vmm.LayoutEx.File))
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("f.Close failure: %v", err)
	}
	err = os.Remove(f.Name())
	if err != nil {
		t.Fatalf("os.Remove(%s) failure: %v", f.Name(), err)
	}

	if err = vm.RefreshStorageInfo(ctx); err != nil {
		t.Error(err)
	}

	if len(vmm.LayoutEx.File) != fileLayoutExCount {
		t.Errorf("expected %d, got %d", fileLayoutExCount, len(vmm.LayoutEx.File))
	}
}

func TestApplyExtraConfig(t *testing.T) {

	applyAndAssertExtraConfigValue := func(
		ctx context.Context,
		vm *object.VirtualMachine,
		val string,
		assertDoesNotExist bool) {

		task, err := vm.Reconfigure(ctx, types.VirtualMachineConfigSpec{
			ExtraConfig: []types.BaseOptionValue{
				&types.OptionValue{
					Key:   "hello",
					Value: val,
				},
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := task.Wait(ctx); err != nil {
			t.Fatal(err)
		}

		var moVM mo.VirtualMachine
		if err := vm.Properties(
			ctx,
			vm.Reference(),
			[]string{"config.extraConfig"},
			&moVM); err != nil {
			t.Fatal(err)
		}
		if moVM.Config == nil {
			t.Fatal("nil config")
		}
		var found bool
		for i := range moVM.Config.ExtraConfig {
			bov := moVM.Config.ExtraConfig[i]
			if bov == nil {
				continue
			}
			ov := bov.GetOptionValue()
			if ov == nil {
				continue
			}
			if ov.Key == "hello" {
				if ov.Value != val {
					t.Fatalf("invalid ExtraConfig value: expected=%s, actual=%v", val, ov.Value)
				}
				found = true
			}
		}
		if !assertDoesNotExist && !found {
			t.Fatal("failed to apply ExtraConfig")
		}
	}

	Test(func(ctx context.Context, c *vim25.Client) {
		vm := object.NewVirtualMachine(c, Map.Any("VirtualMachine").Reference())
		applyAndAssertExtraConfigValue(ctx, vm, "world", false)
		applyAndAssertExtraConfigValue(ctx, vm, "there", false)
		applyAndAssertExtraConfigValue(ctx, vm, "", true)
	})
}

func TestLastModifiedAndChangeVersionAreUpdated(t *testing.T) {
	Test(func(ctx context.Context, c *vim25.Client) {
		vm := object.NewVirtualMachine(c, Map.Any("VirtualMachine").Reference())
		var vmMo mo.VirtualMachine
		if err := vm.Properties(
			ctx,
			vm.Reference(),
			[]string{"config.modified", "config.changeVersion"},
			&vmMo); err != nil {

			t.Fatalf("failed to fetch initial vm props: %v", err)
		}

		oldModified := vmMo.Config.Modified
		oldChangeVersion := vmMo.Config.ChangeVersion

		tsk, err := vm.Reconfigure(ctx, types.VirtualMachineConfigSpec{
			ExtraConfig: []types.BaseOptionValue{
				&types.OptionValue{
					Key:   "hello",
					Value: "world",
				},
			},
		})
		if err != nil {
			t.Fatalf("failed to call reconfigure api: %v", err)
		}
		if err := tsk.WaitEx(ctx); err != nil {
			t.Fatalf("failed to reconfigure: %v", err)
		}

		if err := vm.Properties(
			ctx,
			vm.Reference(),
			[]string{"config.modified", "config.changeVersion"},
			&vmMo); err != nil {

			t.Fatalf("failed to fetch vm props after reconfigure: %v", err)
		}

		newModified := vmMo.Config.Modified
		newChangeVersion := vmMo.Config.ChangeVersion

		if a, e := newModified, oldModified; a == e {
			t.Errorf("config.modified was not updated: %v", a)
		}

		if a, e := newChangeVersion, oldChangeVersion; a == e {
			t.Errorf("config.changeVersion was not updated: %v", a)
		}
	})
}

func TestUpgradeVm(t *testing.T) {

	const (
		vmx1  = "vmx-1"
		vmx2  = "vmx-2"
		vmx15 = "vmx-15"
		vmx17 = "vmx-17"
		vmx19 = "vmx-19"
		vmx20 = "vmx-20"
		vmx21 = "vmx-21"
		vmx22 = "vmx-22"
	)

	model := VPX()
	model.Autostart = false
	model.Cluster = 1
	model.ClusterHost = 1
	model.Host = 1

	Test(func(ctx context.Context, c *vim25.Client) {
		props := []string{"config.version", "summary.config.hwVersion"}

		vm := object.NewVirtualMachine(c, Map.Any("VirtualMachine").Reference())
		vm2 := Map.Get(vm.Reference()).(*VirtualMachine)
		Map.WithLock(SpoofContext(), vm2.Reference(), func() {
			vm2.Config.Version = vmx15
		})

		host, err := vm.HostSystem(ctx)
		if err != nil {
			t.Fatalf("failed to get vm's host: %v", err)
		}
		host2 := Map.Get(host.Reference()).(*HostSystem)

		var eb *EnvironmentBrowser
		{
			ref := Map.Get(host.Reference()).(*HostSystem).Parent
			switch ref.Type {
			case "ClusterComputeResource":
				obj := Map.Get(*ref).(*ClusterComputeResource)
				eb = Map.Get(*obj.EnvironmentBrowser).(*EnvironmentBrowser)
			case "ComputeResource":
				obj := Map.Get(*ref).(*mo.ComputeResource)
				eb = Map.Get(*obj.EnvironmentBrowser).(*EnvironmentBrowser)
			}
		}

		baseline := func() {
			Map.WithLock(SpoofContext(), vm2.Reference(), func() {
				vm2.Config.Version = vmx15
				vm2.Config.Template = false
				vm2.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOff
			})
			Map.WithLock(SpoofContext(), host2.Reference(), func() {
				host2.Runtime.InMaintenanceMode = false
			})
			Map.WithLock(SpoofContext(), eb.Reference(), func() {
				for i := range eb.QueryConfigOptionDescriptorResponse.Returnval {
					cod := &eb.QueryConfigOptionDescriptorResponse.Returnval[i]
					hostFound := false
					for j := range cod.Host {
						if cod.Host[j].Value == host2.Reference().Value {
							hostFound = true
							break
						}
					}
					if !hostFound {
						cod.Host = append(cod.Host, host2.Reference())
					}
				}
			})
		}

		t.Run("InvalidPowerState", func(t *testing.T) {
			baseline()
			Map.WithLock(SpoofContext(), vm2.Reference(), func() {
				vm2.Runtime.PowerState = types.VirtualMachinePowerStatePoweredOn
			})

			tsk, err := vm.UpgradeVM(ctx, vmx15)
			if err != nil {
				t.Fatalf("failed to call upgradeVm api: %v", err)
			}
			if _, err := tsk.WaitForResultEx(ctx); err == nil {
				t.Fatal("expected error did not occur")
			} else if err2, ok := err.(task.Error); !ok {
				t.Fatalf("unexpected error: %[1]T %+[1]v", err)
			} else if f := err2.Fault(); f == nil {
				t.Fatal("fault is nil")
			} else if f2, ok := f.(*types.InvalidPowerStateFault); !ok {
				t.Fatalf("unexpected fault: %[1]T %+[1]v", f)
			} else {
				if f2.ExistingState != types.VirtualMachinePowerStatePoweredOn {
					t.Errorf("unexpected existing state: %v", f2.ExistingState)
				}
				if f2.RequestedState != types.VirtualMachinePowerStatePoweredOff {
					t.Errorf("unexpected requested state: %v", f2.RequestedState)
				}
			}
		})

		t.Run("InvalidState", func(t *testing.T) {
			t.Run("MaintenanceMode", func(t *testing.T) {
				baseline()
				Map.WithLock(SpoofContext(), host2.Reference(), func() {
					host2.Runtime.InMaintenanceMode = true
				})

				if tsk, err := vm.UpgradeVM(ctx, vmx15); err != nil {
					t.Fatalf("failed to call upgradeVm api: %v", err)
				} else if _, err := tsk.WaitForResultEx(ctx); err == nil {
					t.Fatal("expected error did not occur")
				} else if err, ok := err.(task.Error); !ok {
					t.Fatalf("unexpected error: %[1]T %+[1]v", err)
				} else if f := err.Fault(); f == nil {
					t.Fatal("fault is nil")
				} else if f2, ok := f.(*types.InvalidState); !ok {
					t.Fatalf("unexpected fault: %[1]T %+[1]v", f)
				} else if fc := f2.FaultCause; fc == nil {
					t.Fatal("fault cause is nil")
				} else if fc.LocalizedMessage != fmt.Sprintf("%s in maintenance mode", host.Reference().Value) {
					t.Fatalf("unexpected error message: %s", fc.LocalizedMessage)
				}
			})

			t.Run("Template", func(t *testing.T) {
				baseline()
				Map.WithLock(SpoofContext(), vm2.Reference(), func() {
					vm2.Config.Template = true
				})

				if tsk, err := vm.UpgradeVM(ctx, vmx15); err != nil {
					t.Fatalf("failed to call upgradeVm api: %v", err)
				} else if _, err := tsk.WaitForResultEx(ctx); err == nil {
					t.Fatal("expected error did not occur")
				} else if err, ok := err.(task.Error); !ok {
					t.Fatalf("unexpected error: %[1]T %+[1]v", err)
				} else if f := err.Fault(); f == nil {
					t.Fatal("fault is nil")
				} else if f2, ok := f.(*types.InvalidState); !ok {
					t.Fatalf("unexpected fault: %[1]T %+[1]v", f)
				} else if fc := f2.FaultCause; fc == nil {
					t.Fatal("fault cause is nil")
				} else if fc.LocalizedMessage != fmt.Sprintf("%s is template", vm.Reference().Value) {
					t.Fatalf("unexpected error message: %s", fc.LocalizedMessage)
				}
			})

			t.Run("LatestHardwareVersion", func(t *testing.T) {
				baseline()
				Map.WithLock(SpoofContext(), vm.Reference(), func() {
					vm2.Config.Version = vmx21
				})

				if tsk, err := vm.UpgradeVM(ctx, vmx21); err != nil {
					t.Fatalf("failed to call upgradeVm api: %v", err)
				} else if _, err := tsk.WaitForResultEx(ctx); err == nil {
					t.Fatal("expected error did not occur")
				} else if err, ok := err.(task.Error); !ok {
					t.Fatalf("unexpected error: %[1]T %+[1]v", err)
				} else if f := err.Fault(); f == nil {
					t.Fatal("fault is nil")
				} else if f2, ok := f.(*types.InvalidState); !ok {
					t.Fatalf("unexpected fault: %[1]T %+[1]v", f)
				} else if fc := f2.FaultCause; fc == nil {
					t.Fatal("fault cause is nil")
				} else if fc.LocalizedMessage != fmt.Sprintf("%s is latest version", vm.Reference().Value) {
					t.Fatalf("unexpected error message: %s", fc.LocalizedMessage)
				}
			})
		})

		t.Run("NotSupported", func(t *testing.T) {
			t.Run("AtAll", func(t *testing.T) {
				baseline()

				if tsk, err := vm.UpgradeVM(ctx, vmx22); err != nil {
					t.Fatalf("failed to call upgradeVm api: %v", err)
				} else if _, err := tsk.WaitForResultEx(ctx); err == nil {
					t.Fatal("expected error did not occur")
				} else if err, ok := err.(task.Error); !ok {
					t.Fatalf("unexpected error: %[1]T %+[1]v", err)
				} else if f := err.Fault(); f == nil {
					t.Fatal("fault is nil")
				} else if f2, ok := f.(*types.NotSupported); !ok {
					t.Fatalf("unexpected fault: %[1]T %+[1]v", f)
				} else if fc := f2.FaultCause; fc == nil {
					t.Fatal("fault cause is nil")
				} else if fc.LocalizedMessage != "vmx-22 not supported" {
					t.Fatalf("unexpected error message: %s", fc.LocalizedMessage)
				}
			})
			t.Run("OnVmHost", func(t *testing.T) {
				baseline()
				Map.WithLock(SpoofContext(), eb.Reference(), func() {
					for i := range eb.QueryConfigOptionDescriptorResponse.Returnval {
						cod := &eb.QueryConfigOptionDescriptorResponse.Returnval[i]
						if cod.Key == vmx17 {
							cod.Host = nil
						}
					}
				})

				if tsk, err := vm.UpgradeVM(ctx, vmx17); err != nil {
					t.Fatalf("failed to call upgradeVm api: %v", err)
				} else if _, err := tsk.WaitForResultEx(ctx); err == nil {
					t.Fatal("expected error did not occur")
				} else if err, ok := err.(task.Error); !ok {
					t.Fatalf("unexpected error: %[1]T %+[1]v", err)
				} else if f := err.Fault(); f == nil {
					t.Fatal("fault is nil")
				} else if f2, ok := f.(*types.NotSupported); !ok {
					t.Fatalf("unexpected fault: %[1]T %+[1]v", f)
				} else if fc := f2.FaultCause; fc == nil {
					t.Fatal("fault cause is nil")
				} else if fc.LocalizedMessage != "vmx-17 not supported" {
					t.Fatalf("unexpected error message: %s", fc.LocalizedMessage)
				}
			})
		})

		t.Run("AlreadyUpgraded", func(t *testing.T) {
			t.Run("EqualToTargetVersion", func(t *testing.T) {
				baseline()
				if tsk, err := vm.UpgradeVM(ctx, vmx15); err != nil {
					t.Fatalf("failed to call upgradeVm api: %v", err)
				} else if _, err := tsk.WaitForResultEx(ctx); err == nil {
					t.Fatal("expected error did not occur")
				} else if err, ok := err.(task.Error); !ok {
					t.Fatalf("unexpected error: %[1]T %+[1]v", err)
				} else if f := err.Fault(); f == nil {
					t.Fatal("fault is nil")
				} else if _, ok := f.(*types.AlreadyUpgradedFault); !ok {
					t.Fatalf("unexpected fault: %[1]T %+[1]v", f)
				}
			})

			t.Run("GreaterThanTargetVersion", func(t *testing.T) {
				baseline()
				Map.WithLock(SpoofContext(), vm2.Reference(), func() {
					vm2.Config.Version = vmx20
				})
				if tsk, err := vm.UpgradeVM(ctx, vmx17); err != nil {
					t.Fatalf("failed to call upgradeVm api: %v", err)
				} else if _, err := tsk.WaitForResultEx(ctx); err == nil {
					t.Fatal("expected error did not occur")
				} else if err, ok := err.(task.Error); !ok {
					t.Fatalf("unexpected error: %[1]T %+[1]v", err)
				} else if f := err.Fault(); f == nil {
					t.Fatal("fault is nil")
				} else if _, ok := f.(*types.AlreadyUpgradedFault); !ok {
					t.Fatalf("unexpected fault: %[1]T %+[1]v", f)
				}
			})
		})

		t.Run("InvalidArgument", func(t *testing.T) {
			baseline()
			Map.WithLock(SpoofContext(), vm2.Reference(), func() {
				vm2.Config.Version = vmx1
			})
			Map.WithLock(SpoofContext(), eb.Reference(), func() {
				eb.QueryConfigOptionDescriptorResponse.Returnval = append(
					eb.QueryConfigOptionDescriptorResponse.Returnval,
					types.VirtualMachineConfigOptionDescriptor{
						Key:  vmx2,
						Host: []types.ManagedObjectReference{host.Reference()},
					})
			})

			if tsk, err := vm.UpgradeVM(ctx, vmx2); err != nil {
				t.Fatalf("failed to call upgradeVm api: %v", err)
			} else if _, err := tsk.WaitForResultEx(ctx); err == nil {
				t.Fatal("expected error did not occur")
			} else if err, ok := err.(task.Error); !ok {
				t.Fatalf("unexpected error: %[1]T %+[1]v", err)
			} else if f := err.Fault(); f == nil {
				t.Fatal("fault is nil")
			} else if _, ok := f.(*types.InvalidArgument); !ok {
				t.Fatalf("unexpected fault: %[1]T %+[1]v", f)
			}
		})

		t.Run("UpgradeToLatest", func(t *testing.T) {
			baseline()

			if tsk, err := vm.UpgradeVM(ctx, ""); err != nil {
				t.Fatalf("failed to call upgradeVm api: %v", err)
			} else if _, err := tsk.WaitForResultEx(ctx); err != nil {
				t.Fatalf("failed to upgrade vm: %v", err)
			}
			var vmMo mo.VirtualMachine
			if err := vm.Properties(
				ctx,
				vm.Reference(),
				props,
				&vmMo); err != nil {

				t.Fatalf("failed to fetch vm props after upgrade: %v", err)
			}
			if v := vmMo.Config.Version; v != vmx21 {
				t.Fatalf("unexpected config.version %v", v)
			}
			if v := vmMo.Summary.Config.HwVersion; v != vmx21 {
				t.Fatalf("unexpected summary.config.hwVersion %v", v)
			}
		})

		t.Run("UpgradeFrom15To17", func(t *testing.T) {
			const targetVersion = vmx17
			baseline()

			if tsk, err := vm.UpgradeVM(ctx, targetVersion); err != nil {
				t.Fatalf("failed to call upgradeVm api: %v", err)
			} else if _, err := tsk.WaitForResultEx(ctx); err != nil {
				t.Fatalf("failed to upgrade vm: %v", err)
			}
			var vmMo mo.VirtualMachine
			if err := vm.Properties(
				ctx,
				vm.Reference(),
				props,
				&vmMo); err != nil {

				t.Fatalf("failed to fetch vm props after upgrade: %v", err)
			}
			if v := vmMo.Config.Version; v != targetVersion {
				t.Fatalf("unexpected config.version %v", v)
			}
			if v := vmMo.Summary.Config.HwVersion; v != targetVersion {
				t.Fatalf("unexpected summary.config.hwVersion %v", v)
			}
		})

		t.Run("UpgradeFrom17To20", func(t *testing.T) {
			const targetVersion = vmx20
			baseline()
			Map.WithLock(SpoofContext(), vm2.Reference(), func() {
				vm2.Config.Version = vmx17
			})

			if tsk, err := vm.UpgradeVM(ctx, targetVersion); err != nil {
				t.Fatalf("failed to call upgradeVm api: %v", err)
			} else if _, err := tsk.WaitForResultEx(ctx); err != nil {
				t.Fatalf("failed to upgrade vm: %v", err)
			}
			var vmMo mo.VirtualMachine
			if err := vm.Properties(
				ctx,
				vm.Reference(),
				props,
				&vmMo); err != nil {

				t.Fatalf("failed to fetch vm props after upgrade: %v", err)
			}
			if v := vmMo.Config.Version; v != targetVersion {
				t.Fatalf("unexpected config.version %v", v)
			}
			if v := vmMo.Summary.Config.HwVersion; v != targetVersion {
				t.Fatalf("unexpected summary.config.hwVersion %v", v)
			}
		})

		t.Run("UpgradeFrom15To17To20", func(t *testing.T) {
			const (
				targetVersion1 = vmx17
				targetVersion2 = vmx20
			)
			baseline()

			if tsk, err := vm.UpgradeVM(ctx, targetVersion1); err != nil {
				t.Fatalf("failed to call upgradeVm api first time: %v", err)
			} else if _, err := tsk.WaitForResultEx(ctx); err != nil {
				t.Fatalf("failed to upgrade vm first time: %v", err)
			}
			var vmMo mo.VirtualMachine
			if err := vm.Properties(
				ctx,
				vm.Reference(),
				props,
				&vmMo); err != nil {

				t.Fatalf("failed to fetch vm props after first upgrade: %v", err)
			}
			if v := vmMo.Config.Version; v != targetVersion1 {
				t.Fatalf("unexpected config.version after first upgrade %v", v)
			}
			if v := vmMo.Summary.Config.HwVersion; v != targetVersion1 {
				t.Fatalf("unexpected summary.config.hwVersion after first upgrade %v", v)
			}

			if tsk, err := vm.UpgradeVM(ctx, targetVersion2); err != nil {
				t.Fatalf("failed to call upgradeVm api second time: %v", err)
			} else if _, err := tsk.WaitForResultEx(ctx); err != nil {
				t.Fatalf("failed to upgrade vm second time: %v", err)
			}
			if err := vm.Properties(
				ctx,
				vm.Reference(),
				props,
				&vmMo); err != nil {

				t.Fatalf("failed to fetch vm props after second upgrade: %v", err)
			}
			if v := vmMo.Config.Version; v != targetVersion2 {
				t.Fatalf("unexpected config.version after second upgrade %v", v)
			}
			if v := vmMo.Summary.Config.HwVersion; v != targetVersion2 {
				t.Fatalf("unexpected summary.config.hwVersion after second upgrade %v", v)
			}
		})

	}, model)
}
