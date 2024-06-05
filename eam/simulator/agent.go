/*
Copyright (c) 2021 VMware, Inc. All Rights Reserved.

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
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/zhengkes/govmomi/simulator"
	vimmethods "github.com/zhengkes/govmomi/vim25/methods"
	"github.com/zhengkes/govmomi/vim25/soap"
	vim "github.com/zhengkes/govmomi/vim25/types"

	"github.com/zhengkes/govmomi/eam/internal"
	"github.com/zhengkes/govmomi/eam/methods"
	"github.com/zhengkes/govmomi/eam/mo"
	"github.com/zhengkes/govmomi/eam/types"
)

// Agenct is the vSphere ESX Agent Manager managed object responsible
// fordeploying an Agency on a single host. The Agent maintains the state
// of the current deployment in its runtime information
type Agent struct {
	EamObject
	mo.Agent
}

type AgentVMPlacementOptions struct {
	computeResource vim.ManagedObjectReference
	datacenter      vim.ManagedObjectReference
	datastore       vim.ManagedObjectReference
	folder          vim.ManagedObjectReference
	host            vim.ManagedObjectReference
	network         vim.ManagedObjectReference
	pool            vim.ManagedObjectReference
}

// NewAgent returns a new Agent as if CreateAgency were called on the
// EsxAgentManager object.
func NewAgent(
	ctx *simulator.Context,
	agency vim.ManagedObjectReference,
	config types.AgentConfigInfo,
	vmName string,
	vmPlacement AgentVMPlacementOptions) (*Agent, vim.BaseMethodFault) {
	vimMap := simulator.Map

	agent := &Agent{
		EamObject: EamObject{
			Self: vim.ManagedObjectReference{
				Type:  internal.Agent,
				Value: uuid.New().String(),
			},
		},
		Agent: mo.Agent{
			Config: config,
			Runtime: types.AgentRuntimeInfo{
				Agency:               &agency,
				VmName:               vmName,
				Host:                 &vmPlacement.host,
				EsxAgentFolder:       &vmPlacement.folder,
				EsxAgentResourcePool: &vmPlacement.pool,
			},
		},
	}

	// Register the agent with the registry in order for the agent to start
	// receiving API calls from clients.
	ctx.Map.Put(agent)

	// simulator.VirtualMachine related calls need the vimMap (aka global Map)
	vimCtx := simulator.SpoofContext()

	createVm := func() (vim.ManagedObjectReference, *vim.LocalizedMethodFault) {
		var vmRef vim.ManagedObjectReference

		// vmExtraConfig is used when creating the VM for this agent.
		vmExtraConfig := []vim.BaseOptionValue{}

		// If config.OvfPackageUrl is non-empty and does not appear to point to
		// a local file or an HTTP URI, then assume it is a container.
		if url := config.OvfPackageUrl; url != "" && !fsOrHTTPRx.MatchString(url) {
			vmExtraConfig = append(
				vmExtraConfig,
				&vim.OptionValue{
					Key:   "RUN.container",
					Value: url,
				})
		}

		// Copy the OVF environment properties into the VM's ExtraConfig property.
		if ovfEnv := config.OvfEnvironment; ovfEnv != nil {
			for _, ovfProp := range ovfEnv.OvfProperty {
				vmExtraConfig = append(
					vmExtraConfig,
					&vim.OptionValue{
						Key:   ovfProp.Key,
						Value: ovfProp.Value,
					})
			}
		}

		datastore := vimMap.Get(vmPlacement.datastore).(*simulator.Datastore)
		vmPathName := fmt.Sprintf("[%[1]s] %[2]s/%[2]s.vmx", datastore.Name, vmName)
		vmConfigSpec := vim.VirtualMachineConfigSpec{
			Name:        vmName,
			ExtraConfig: vmExtraConfig,
			Files: &vim.VirtualMachineFileInfo{
				VmPathName: vmPathName,
			},
		}

		// Create the VM for this agent.
		vmFolder := vimMap.Get(vmPlacement.folder).(*simulator.Folder)
		createVmTaskRef := vmFolder.CreateVMTask(vimCtx, &vim.CreateVM_Task{
			This:   vmFolder.Self,
			Config: vmConfigSpec,
			Pool:   vmPlacement.pool,
			Host:   &vmPlacement.host,
		}).(*vimmethods.CreateVM_TaskBody).Res.Returnval
		createVmTask := simulator.Map.Get(createVmTaskRef).(*simulator.Task)

		// Wait for the task to complete and see if there is an error.
		createVmTask.Wait()
		if createVmTask.Info.Error != nil {
			return vmRef, createVmTask.Info.Error
		}

		vmRef = createVmTask.Info.Result.(vim.ManagedObjectReference)
		vm := vimMap.Get(vmRef).(*simulator.VirtualMachine)
		log.Printf("created agent vm: MoRef=%v, Name=%s", vm.Self, vm.Name)

		// Link the agent to this VM.
		agent.Runtime.Vm = &vm.Self

		return vm.Self, nil
	}

	vmRef, err := createVm()
	if err != nil {
		return nil, &vim.RuntimeFault{
			MethodFault: vim.MethodFault{
				FaultCause: err,
			},
		}
	}

	// Start watching this VM and updating the agent's information about the VM.
	go func(ctx *simulator.Context, eamReg, vimReg *simulator.Registry) {
		var (
			ticker = time.NewTicker(1 * time.Second)
			vmName string
		)
		for range ticker.C {
			eamReg.WithLock(ctx, agent.Self, func() {
				agentObj := eamReg.Get(agent.Self)
				if agentObj == nil {
					log.Printf("not found: %v", agent.Self)
					// If the agent no longer exists then stop watching it.
					ticker.Stop()
					return
				}

				updateAgent := func(vm *simulator.VirtualMachine) {
					if vmName == "" {
						vmName = vm.Config.Name
					}

					// Update the agent's properties from the VM.
					agent := agentObj.(*Agent)
					agent.Runtime.VmPowerState = vm.Runtime.PowerState
					if guest := vm.Summary.Guest; guest == nil {
						agent.Runtime.VmIp = ""
					} else {
						agent.Runtime.VmIp = guest.IpAddress
					}
				}

				vimReg.WithLock(ctx, vmRef, func() {
					if vmObj := vimReg.Get(vmRef); vmObj != nil {
						updateAgent(vmObj.(*simulator.VirtualMachine))
					} else {
						// If the VM no longer exists then create a new agent VM.
						log.Printf(
							"creating new agent vm: %v, %v, vmName=%s",
							agent.Self, vmRef, vmName)

						newVmRef, err := createVm()
						if err != nil {
							log.Printf(
								"failed to create new agent vm: %v, %v, vmName=%s, err=%v",
								agent.Self, vmRef, vmName, *err)
							ticker.Stop()
							return
						}

						// Make sure the vmRef variable is assigned to the new
						// VM's reference for the next time through this loop.
						vmRef = newVmRef

						// Get a lock for the *new* VM.
						vimReg.WithLock(ctx, vmRef, func() {
							vmObj = vimReg.Get(vmRef)
							if vmObj == nil {
								log.Printf("not found: %v", vmRef)
								ticker.Stop()
								return
							}
							updateAgent(vmObj.(*simulator.VirtualMachine))
						})
					}

				})
			})
		}
	}(simulator.SpoofContext(), ctx.Map, vimMap)

	return agent, nil
}

func (m *Agent) AgentQueryConfig(
	ctx *simulator.Context,
	req *types.AgentQueryConfig) soap.HasFault {

	return &methods.AgentQueryConfigBody{
		Res: &types.AgentQueryConfigResponse{
			Returnval: m.Config,
		},
	}
}

func (m *Agent) AgentQueryRuntime(
	ctx *simulator.Context,
	req *types.AgentQueryRuntime) soap.HasFault {

	return &methods.AgentQueryRuntimeBody{
		Res: &types.AgentQueryRuntimeResponse{
			Returnval: m.Runtime,
		},
	}
}

func (m *Agent) MarkAsAvailable(
	ctx *simulator.Context,
	req *types.MarkAsAvailable) soap.HasFault {

	return &methods.MarkAsAvailableBody{
		Res: &types.MarkAsAvailableResponse{},
	}
}
