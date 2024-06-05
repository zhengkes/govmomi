/*
Copyright (c) 2019 VMware, Inc. All Rights Reserved.

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
	"reflect"
	"time"

	"github.com/google/uuid"

	"github.com/zhengkes/govmomi/cns"
	"github.com/zhengkes/govmomi/cns/methods"
	cnstypes "github.com/zhengkes/govmomi/cns/types"
	pbmtypes "github.com/zhengkes/govmomi/pbm/types"
	"github.com/zhengkes/govmomi/simulator"
	"github.com/zhengkes/govmomi/vim25/soap"
	vim25types "github.com/zhengkes/govmomi/vim25/types"
)

func init() {
	simulator.RegisterEndpoint(func(s *simulator.Service, r *simulator.Registry) {
		if r.IsVPX() {
			s.RegisterSDK(New())
		}
	})
}

func New() *simulator.Registry {
	r := simulator.NewRegistry()
	r.Namespace = cns.Namespace
	r.Path = cns.Path

	r.Put(&CnsVolumeManager{
		ManagedObjectReference: cns.CnsVolumeManagerInstance,
		volumes:                make(map[vim25types.ManagedObjectReference]map[cnstypes.CnsVolumeId]*cnstypes.CnsVolume),
		attachments:            make(map[cnstypes.CnsVolumeId]vim25types.ManagedObjectReference),
		snapshots:              make(map[cnstypes.CnsVolumeId]map[cnstypes.CnsSnapshotId]*cnstypes.CnsSnapshot),
	})

	return r
}

type CnsVolumeManager struct {
	vim25types.ManagedObjectReference
	volumes     map[vim25types.ManagedObjectReference]map[cnstypes.CnsVolumeId]*cnstypes.CnsVolume
	attachments map[cnstypes.CnsVolumeId]vim25types.ManagedObjectReference
	snapshots   map[cnstypes.CnsVolumeId]map[cnstypes.CnsSnapshotId]*cnstypes.CnsSnapshot
}

const simulatorDiskUUID = "6000c298595bf4575739e9105b2c0c2d"

func (m *CnsVolumeManager) CnsCreateVolume(ctx *simulator.Context, req *cnstypes.CnsCreateVolume) soap.HasFault {
	task := simulator.CreateTask(m, "CnsCreateVolume", func(*simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		if len(req.CreateSpecs) == 0 {
			return nil, &vim25types.InvalidArgument{InvalidProperty: "CnsVolumeCreateSpec"}
		}

		operationResult := []cnstypes.BaseCnsVolumeOperationResult{}
		for _, createSpec := range req.CreateSpecs {
			staticProvisionedSpec, ok := interface{}(createSpec.BackingObjectDetails).(*cnstypes.CnsBlockBackingDetails)
			if ok && staticProvisionedSpec.BackingDiskId != "" {
				datastore := simulator.Map.Any("Datastore").(*simulator.Datastore)
				volumes, ok := m.volumes[datastore.Self]
				if !ok {
					volumes = make(map[cnstypes.CnsVolumeId]*cnstypes.CnsVolume)
					m.volumes[datastore.Self] = volumes
				}
				newVolume := &cnstypes.CnsVolume{
					VolumeId: cnstypes.CnsVolumeId{
						Id: interface{}(createSpec.BackingObjectDetails).(*cnstypes.CnsBlockBackingDetails).BackingDiskId,
					},
					Name:                         createSpec.Name,
					VolumeType:                   createSpec.VolumeType,
					DatastoreUrl:                 datastore.Info.GetDatastoreInfo().Url,
					Metadata:                     createSpec.Metadata,
					BackingObjectDetails:         createSpec.BackingObjectDetails.(cnstypes.BaseCnsBackingObjectDetails).GetCnsBackingObjectDetails(),
					ComplianceStatus:             "Simulator Compliance Status",
					DatastoreAccessibilityStatus: "Simulator Datastore Accessibility Status",
					HealthStatus:                 string(pbmtypes.PbmHealthStatusForEntityGreen),
				}

				volumes[newVolume.VolumeId] = newVolume
				placementResults := []cnstypes.CnsPlacementResult{}
				placementResults = append(placementResults, cnstypes.CnsPlacementResult{
					Datastore: datastore.Reference(),
				})
				operationResult = append(operationResult, &cnstypes.CnsVolumeCreateResult{
					CnsVolumeOperationResult: cnstypes.CnsVolumeOperationResult{
						VolumeId: newVolume.VolumeId,
					},
					Name:             createSpec.Name,
					PlacementResults: placementResults,
				})

			} else {
				for _, datastoreRef := range createSpec.Datastores {
					datastore := simulator.Map.Get(datastoreRef).(*simulator.Datastore)

					volumes, ok := m.volumes[datastore.Self]
					if !ok {
						volumes = make(map[cnstypes.CnsVolumeId]*cnstypes.CnsVolume)
						m.volumes[datastore.Self] = volumes

					}

					var policyId string
					if createSpec.Profile != nil && createSpec.Profile[0] != nil &&
						reflect.TypeOf(createSpec.Profile[0]) == reflect.TypeOf(&vim25types.VirtualMachineDefinedProfileSpec{}) {
						policyId = interface{}(createSpec.Profile[0]).(*vim25types.VirtualMachineDefinedProfileSpec).ProfileId
					}

					newVolume := &cnstypes.CnsVolume{
						VolumeId: cnstypes.CnsVolumeId{
							Id: uuid.New().String(),
						},
						Name:                         createSpec.Name,
						VolumeType:                   createSpec.VolumeType,
						DatastoreUrl:                 datastore.Info.GetDatastoreInfo().Url,
						Metadata:                     createSpec.Metadata,
						BackingObjectDetails:         createSpec.BackingObjectDetails.(cnstypes.BaseCnsBackingObjectDetails).GetCnsBackingObjectDetails(),
						ComplianceStatus:             "Simulator Compliance Status",
						DatastoreAccessibilityStatus: "Simulator Datastore Accessibility Status",
						HealthStatus:                 string(pbmtypes.PbmHealthStatusForEntityGreen),
						StoragePolicyId:              policyId,
					}

					volumes[newVolume.VolumeId] = newVolume
					placementResults := []cnstypes.CnsPlacementResult{}
					placementResults = append(placementResults, cnstypes.CnsPlacementResult{
						Datastore: datastore.Reference(),
					})
					operationResult = append(operationResult, &cnstypes.CnsVolumeCreateResult{
						CnsVolumeOperationResult: cnstypes.CnsVolumeOperationResult{
							VolumeId: newVolume.VolumeId,
						},
						Name:             createSpec.Name,
						PlacementResults: placementResults,
					})
				}
			}
		}

		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: operationResult,
		}, nil
	})

	return &methods.CnsCreateVolumeBody{
		Res: &cnstypes.CnsCreateVolumeResponse{
			Returnval: task.Run(ctx),
		},
	}
}

// CnsQueryVolume simulates the query volumes implementation for CNSQuery API
func (m *CnsVolumeManager) CnsQueryVolume(ctx context.Context, req *cnstypes.CnsQueryVolume) soap.HasFault {
	retVolumes := []cnstypes.CnsVolume{}
	reqVolumeIds := make(map[string]bool)
	isQueryFilter := false

	if req.Filter.VolumeIds != nil {
		isQueryFilter = true
	}
	// Create map of requested volume Ids in query request
	for _, volumeID := range req.Filter.VolumeIds {
		reqVolumeIds[volumeID.Id] = true
	}

	for _, dsVolumes := range m.volumes {
		for _, volume := range dsVolumes {
			if isQueryFilter {
				if _, ok := reqVolumeIds[volume.VolumeId.Id]; ok {
					retVolumes = append(retVolumes, *volume)
				}
			} else {
				retVolumes = append(retVolumes, *volume)
			}
		}
	}

	return &methods.CnsQueryVolumeBody{
		Res: &cnstypes.CnsQueryVolumeResponse{
			Returnval: cnstypes.CnsQueryResult{
				Volumes: retVolumes,
				Cursor:  cnstypes.CnsCursor{},
			},
		},
	}
}

// CnsQueryAllVolume simulates the query volumes implementation for CNSQueryAll API
func (m *CnsVolumeManager) CnsQueryAllVolume(ctx context.Context, req *cnstypes.CnsQueryAllVolume) soap.HasFault {
	retVolumes := []cnstypes.CnsVolume{}
	reqVolumeIds := make(map[string]bool)
	isQueryFilter := false

	if req.Filter.VolumeIds != nil {
		isQueryFilter = true
	}
	// Create map of requested volume Ids in query request
	for _, volumeID := range req.Filter.VolumeIds {
		reqVolumeIds[volumeID.Id] = true
	}

	for _, dsVolumes := range m.volumes {
		for _, volume := range dsVolumes {
			if isQueryFilter {
				if _, ok := reqVolumeIds[volume.VolumeId.Id]; ok {
					retVolumes = append(retVolumes, *volume)
				}
			} else {
				retVolumes = append(retVolumes, *volume)
			}
		}
	}

	return &methods.CnsQueryAllVolumeBody{
		Res: &cnstypes.CnsQueryAllVolumeResponse{
			Returnval: cnstypes.CnsQueryResult{
				Volumes: retVolumes,
				Cursor:  cnstypes.CnsCursor{},
			},
		},
	}
}

func (m *CnsVolumeManager) CnsDeleteVolume(ctx *simulator.Context, req *cnstypes.CnsDeleteVolume) soap.HasFault {
	task := simulator.CreateTask(m, "CnsDeleteVolume", func(*simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		operationResult := []cnstypes.BaseCnsVolumeOperationResult{}
		for _, volumeId := range req.VolumeIds {
			for ds, dsVolumes := range m.volumes {
				volume := dsVolumes[volumeId]
				if volume != nil {
					delete(m.volumes[ds], volumeId)
					operationResult = append(operationResult, &cnstypes.CnsVolumeOperationResult{
						VolumeId: volumeId,
					})

				}
			}
		}
		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: operationResult,
		}, nil
	})

	return &methods.CnsDeleteVolumeBody{
		Res: &cnstypes.CnsDeleteVolumeResponse{
			Returnval: task.Run(ctx),
		},
	}
}

// CnsUpdateVolumeMetadata simulates UpdateVolumeMetadata call for simulated vc
func (m *CnsVolumeManager) CnsUpdateVolumeMetadata(ctx *simulator.Context, req *cnstypes.CnsUpdateVolumeMetadata) soap.HasFault {
	task := simulator.CreateTask(m, "CnsUpdateVolumeMetadata", func(*simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		if len(req.UpdateSpecs) == 0 {
			return nil, &vim25types.InvalidArgument{InvalidProperty: "CnsUpdateVolumeMetadataSpec"}
		}
		operationResult := []cnstypes.BaseCnsVolumeOperationResult{}
		for _, updateSpecs := range req.UpdateSpecs {
			for _, dsVolumes := range m.volumes {
				for id, volume := range dsVolumes {
					if id.Id == updateSpecs.VolumeId.Id {
						volume.Metadata.EntityMetadata = updateSpecs.Metadata.EntityMetadata
						operationResult = append(operationResult, &cnstypes.CnsVolumeOperationResult{
							VolumeId: volume.VolumeId,
						})
						break
					}
				}
			}

		}
		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: operationResult,
		}, nil
	})
	return &methods.CnsUpdateVolumeBody{
		Res: &cnstypes.CnsUpdateVolumeMetadataResponse{
			Returnval: task.Run(ctx),
		},
	}
}

// CnsAttachVolume simulates AttachVolume call for simulated vc
func (m *CnsVolumeManager) CnsAttachVolume(ctx *simulator.Context, req *cnstypes.CnsAttachVolume) soap.HasFault {
	task := simulator.CreateTask(m, "CnsAttachVolume", func(task *simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		if len(req.AttachSpecs) == 0 {
			return nil, &vim25types.InvalidArgument{InvalidProperty: "CnsAttachVolumeSpec"}
		}
		operationResult := []cnstypes.BaseCnsVolumeOperationResult{}
		for _, attachSpec := range req.AttachSpecs {
			node := simulator.Map.Get(attachSpec.Vm).(*simulator.VirtualMachine)
			if _, ok := m.attachments[attachSpec.VolumeId]; !ok {
				m.attachments[attachSpec.VolumeId] = node.Self
			} else {
				return nil, &vim25types.ResourceInUse{
					Name: attachSpec.VolumeId.Id,
				}
			}
			operationResult = append(operationResult, &cnstypes.CnsVolumeAttachResult{
				CnsVolumeOperationResult: cnstypes.CnsVolumeOperationResult{
					VolumeId: attachSpec.VolumeId,
				},
				DiskUUID: simulatorDiskUUID,
			})
		}

		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: operationResult,
		}, nil
	})

	return &methods.CnsAttachVolumeBody{
		Res: &cnstypes.CnsAttachVolumeResponse{
			Returnval: task.Run(ctx),
		},
	}
}

// CnsDetachVolume simulates DetachVolume call for simulated vc
func (m *CnsVolumeManager) CnsDetachVolume(ctx *simulator.Context, req *cnstypes.CnsDetachVolume) soap.HasFault {
	task := simulator.CreateTask(m, "CnsDetachVolume", func(*simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		if len(req.DetachSpecs) == 0 {
			return nil, &vim25types.InvalidArgument{InvalidProperty: "CnsDetachVolumeSpec"}
		}
		operationResult := []cnstypes.BaseCnsVolumeOperationResult{}
		for _, detachSpec := range req.DetachSpecs {
			if _, ok := m.attachments[detachSpec.VolumeId]; ok {
				delete(m.attachments, detachSpec.VolumeId)
				operationResult = append(operationResult, &cnstypes.CnsVolumeOperationResult{
					VolumeId: detachSpec.VolumeId,
				})
			} else {
				return nil, &vim25types.InvalidArgument{
					InvalidProperty: detachSpec.VolumeId.Id,
				}
			}
		}

		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: operationResult,
		}, nil
	})
	return &methods.CnsDetachVolumeBody{
		Res: &cnstypes.CnsDetachVolumeResponse{
			Returnval: task.Run(ctx),
		},
	}
}

// CnsExtendVolume simulates ExtendVolume call for simulated vc
func (m *CnsVolumeManager) CnsExtendVolume(ctx *simulator.Context, req *cnstypes.CnsExtendVolume) soap.HasFault {
	task := simulator.CreateTask(m, "CnsExtendVolume", func(task *simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		if len(req.ExtendSpecs) == 0 {
			return nil, &vim25types.InvalidArgument{InvalidProperty: "CnsExtendVolumeSpec"}
		}
		operationResult := []cnstypes.BaseCnsVolumeOperationResult{}

		for _, extendSpecs := range req.ExtendSpecs {
			for _, dsVolumes := range m.volumes {
				for id, volume := range dsVolumes {
					if id.Id == extendSpecs.VolumeId.Id {
						volume.BackingObjectDetails = &cnstypes.CnsBackingObjectDetails{
							CapacityInMb: extendSpecs.CapacityInMb,
						}
						operationResult = append(operationResult, &cnstypes.CnsVolumeOperationResult{
							VolumeId: volume.VolumeId,
						})
						break
					}
				}
			}
		}

		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: operationResult,
		}, nil
	})

	return &methods.CnsExtendVolumeBody{
		Res: &cnstypes.CnsExtendVolumeResponse{
			Returnval: task.Run(ctx),
		},
	}
}

func (m *CnsVolumeManager) CnsQueryVolumeInfo(ctx *simulator.Context, req *cnstypes.CnsQueryVolumeInfo) soap.HasFault {
	task := simulator.CreateTask(m, "CnsQueryVolumeInfo", func(*simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		operationResult := []cnstypes.BaseCnsVolumeOperationResult{}
		for _, volumeId := range req.VolumeIds {
			vstorageObject := vim25types.VStorageObject{
				Config: vim25types.VStorageObjectConfigInfo{
					BaseConfigInfo: vim25types.BaseConfigInfo{
						Id: vim25types.ID{
							Id: uuid.New().String(),
						},
						Name:                        "name",
						CreateTime:                  time.Now(),
						KeepAfterDeleteVm:           vim25types.NewBool(true),
						RelocationDisabled:          vim25types.NewBool(false),
						NativeSnapshotSupported:     vim25types.NewBool(false),
						ChangedBlockTrackingEnabled: vim25types.NewBool(false),
						Iofilter:                    nil,
					},
					CapacityInMB:    1024,
					ConsumptionType: []string{"disk"},
					ConsumerId:      nil,
				},
			}
			vstorageObject.Config.Backing = &vim25types.BaseConfigInfoDiskFileBackingInfo{
				BaseConfigInfoFileBackingInfo: vim25types.BaseConfigInfoFileBackingInfo{
					BaseConfigInfoBackingInfo: vim25types.BaseConfigInfoBackingInfo{
						Datastore: simulator.Map.Any("Datastore").(*simulator.Datastore).Self,
					},
					FilePath:        "[vsanDatastore] 6785a85e-268e-6352-a2e8-02008b7afadd/kubernetes-dynamic-pvc-68734c9f-a679-42e6-a694-39632c51e31f.vmdk",
					BackingObjectId: volumeId.Id,
					Parent:          nil,
					DeltaSizeInMB:   0,
				},
			}

			operationResult = append(operationResult, &cnstypes.CnsQueryVolumeInfoResult{
				CnsVolumeOperationResult: cnstypes.CnsVolumeOperationResult{
					VolumeId: volumeId,
				},
				VolumeInfo: &cnstypes.CnsBlockVolumeInfo{
					CnsVolumeInfo:  cnstypes.CnsVolumeInfo{},
					VStorageObject: vstorageObject,
				},
			})

		}
		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: operationResult,
		}, nil
	})

	return &methods.CnsQueryVolumeInfoBody{
		Res: &cnstypes.CnsQueryVolumeInfoResponse{
			Returnval: task.Run(ctx),
		},
	}
}

func (m *CnsVolumeManager) CnsQueryAsync(ctx *simulator.Context, req *cnstypes.CnsQueryAsync) soap.HasFault {
	task := simulator.CreateTask(m, "QueryVolumeAsync", func(*simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		retVolumes := []cnstypes.CnsVolume{}
		reqVolumeIds := make(map[string]bool)
		isQueryFilter := false

		if req.Filter.VolumeIds != nil {
			isQueryFilter = true
		}
		// Create map of requested volume Ids in query request
		for _, volumeID := range req.Filter.VolumeIds {
			reqVolumeIds[volumeID.Id] = true
		}

		for _, dsVolumes := range m.volumes {
			for _, volume := range dsVolumes {
				if isQueryFilter {
					if _, ok := reqVolumeIds[volume.VolumeId.Id]; ok {
						retVolumes = append(retVolumes, *volume)
					}
				} else {
					retVolumes = append(retVolumes, *volume)
				}
			}
		}
		operationResult := []cnstypes.BaseCnsVolumeOperationResult{}
		operationResult = append(operationResult, &cnstypes.CnsAsyncQueryResult{
			QueryResult: cnstypes.CnsQueryResult{
				Volumes: retVolumes,
				Cursor:  cnstypes.CnsCursor{},
			},
		})

		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: operationResult,
		}, nil
	})

	return &methods.CnsQueryAsyncBody{
		Res: &cnstypes.CnsQueryAsyncResponse{
			Returnval: task.Run(ctx),
		},
	}
}

func (m *CnsVolumeManager) CnsCreateSnapshots(ctx *simulator.Context, req *cnstypes.CnsCreateSnapshots) soap.HasFault {
	task := simulator.CreateTask(m, "CreateSnapshots", func(*simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		if len(req.SnapshotSpecs) == 0 {
			return nil, &vim25types.InvalidArgument{InvalidProperty: "CnsSnapshotCreateSpec"}
		}

		snapshotOperationResult := []cnstypes.BaseCnsVolumeOperationResult{}
		for _, snapshotCreateSpec := range req.SnapshotSpecs {
			for _, dsVolumes := range m.volumes {
				for id, _ := range dsVolumes {
					if id.Id != snapshotCreateSpec.VolumeId.Id {
						continue
					}
					snapshots, ok := m.snapshots[snapshotCreateSpec.VolumeId]
					if !ok {
						snapshots = make(map[cnstypes.CnsSnapshotId]*cnstypes.CnsSnapshot)
						m.snapshots[snapshotCreateSpec.VolumeId] = snapshots
					}

					newSnapshot := &cnstypes.CnsSnapshot{
						SnapshotId: cnstypes.CnsSnapshotId{
							Id: uuid.New().String(),
						},
						VolumeId:    snapshotCreateSpec.VolumeId,
						Description: snapshotCreateSpec.Description,
						CreateTime:  time.Now(),
					}
					snapshots[newSnapshot.SnapshotId] = newSnapshot
					snapshotOperationResult = append(snapshotOperationResult, &cnstypes.CnsSnapshotCreateResult{
						CnsSnapshotOperationResult: cnstypes.CnsSnapshotOperationResult{
							CnsVolumeOperationResult: cnstypes.CnsVolumeOperationResult{
								VolumeId: newSnapshot.VolumeId,
							},
						},
						Snapshot: *newSnapshot,
					})
				}
			}
		}

		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: snapshotOperationResult,
		}, nil
	})

	return &methods.CnsCreateSnapshotsBody{
		Res: &cnstypes.CnsCreateSnapshotsResponse{
			Returnval: task.Run(ctx),
		},
	}
}

func (m *CnsVolumeManager) CnsDeleteSnapshots(ctx *simulator.Context, req *cnstypes.CnsDeleteSnapshots) soap.HasFault {
	task := simulator.CreateTask(m, "DeleteSnapshots", func(*simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		snapshotOperationResult := []cnstypes.BaseCnsVolumeOperationResult{}
		for _, snapshotDeleteSpec := range req.SnapshotDeleteSpecs {
			for _, dsVolumes := range m.volumes {
				for id, _ := range dsVolumes {
					if id.Id != snapshotDeleteSpec.VolumeId.Id {
						continue
					}
					snapshots := m.snapshots[snapshotDeleteSpec.VolumeId]
					snapshot, ok := snapshots[snapshotDeleteSpec.SnapshotId]
					if ok {
						delete(m.snapshots[snapshotDeleteSpec.VolumeId], snapshotDeleteSpec.SnapshotId)
						snapshotOperationResult = append(snapshotOperationResult, &cnstypes.CnsSnapshotDeleteResult{
							CnsSnapshotOperationResult: cnstypes.CnsSnapshotOperationResult{
								CnsVolumeOperationResult: cnstypes.CnsVolumeOperationResult{
									VolumeId: snapshot.VolumeId,
								},
							},
							SnapshotId: snapshot.SnapshotId,
						})
					}
				}
			}
		}

		return &cnstypes.CnsVolumeOperationBatchResult{
			VolumeResults: snapshotOperationResult,
		}, nil
	})

	return &methods.CnsDeleteSnapshotBody{
		Res: &cnstypes.CnsDeleteSnapshotsResponse{
			Returnval: task.Run(ctx),
		},
	}
}

func (m *CnsVolumeManager) CnsQuerySnapshots(ctx *simulator.Context, req *cnstypes.CnsQuerySnapshots) soap.HasFault {
	task := simulator.CreateTask(m, "QuerySnapshots", func(*simulator.Task) (vim25types.AnyType, vim25types.BaseMethodFault) {
		if len(req.SnapshotQueryFilter.SnapshotQuerySpecs) > 1 {
			return nil, &vim25types.InvalidArgument{InvalidProperty: "CnsSnapshotQuerySpec"}
		}

		snapshotQueryResultEntries := []cnstypes.CnsSnapshotQueryResultEntry{}
		checkVolumeExists := func(volumeId cnstypes.CnsVolumeId) bool {
			for _, dsVolumes := range m.volumes {
				for id, _ := range dsVolumes {
					if id.Id == volumeId.Id {
						return true
					}
				}
			}
			return false
		}

		if req.SnapshotQueryFilter.SnapshotQuerySpecs == nil && len(req.SnapshotQueryFilter.SnapshotQuerySpecs) == 0 {
			// return all snapshots if snapshotQuerySpecs is empty
			for _, volSnapshots := range m.snapshots {
				for _, snapshot := range volSnapshots {
					snapshotQueryResultEntries = append(snapshotQueryResultEntries, cnstypes.CnsSnapshotQueryResultEntry{Snapshot: *snapshot})
				}
			}
		} else {
			// snapshotQuerySpecs is not empty
			isSnapshotQueryFilter := false
			snapshotQuerySpec := req.SnapshotQueryFilter.SnapshotQuerySpecs[0]
			if snapshotQuerySpec.SnapshotId != nil && (*snapshotQuerySpec.SnapshotId != cnstypes.CnsSnapshotId{}) {
				isSnapshotQueryFilter = true
			}

			if !checkVolumeExists(snapshotQuerySpec.VolumeId) {
				// volumeId in snapshotQuerySpecs does not exist
				snapshotQueryResultEntries = append(snapshotQueryResultEntries, cnstypes.CnsSnapshotQueryResultEntry{
					Error: &vim25types.LocalizedMethodFault{
						Fault: cnstypes.CnsVolumeNotFoundFault{
							VolumeId: snapshotQuerySpec.VolumeId,
						},
					},
				})
			} else {
				// volumeId in snapshotQuerySpecs exists
				for _, snapshot := range m.snapshots[snapshotQuerySpec.VolumeId] {
					if isSnapshotQueryFilter && snapshot.SnapshotId.Id != (*snapshotQuerySpec.SnapshotId).Id {
						continue
					}

					snapshotQueryResultEntries = append(snapshotQueryResultEntries, cnstypes.CnsSnapshotQueryResultEntry{Snapshot: *snapshot})
				}

				if isSnapshotQueryFilter && len(snapshotQueryResultEntries) == 0 {
					snapshotQueryResultEntries = append(snapshotQueryResultEntries, cnstypes.CnsSnapshotQueryResultEntry{
						Error: &vim25types.LocalizedMethodFault{
							Fault: cnstypes.CnsSnapshotNotFoundFault{
								VolumeId:   snapshotQuerySpec.VolumeId,
								SnapshotId: *snapshotQuerySpec.SnapshotId,
							},
						},
					})
				}
			}
		}

		return &cnstypes.CnsSnapshotQueryResult{
			Entries: snapshotQueryResultEntries,
		}, nil
	})

	return &methods.CnsQuerySnapshotsBody{
		Res: &cnstypes.CnsQuerySnapshotsResponse{
			Returnval: task.Run(ctx),
		},
	}
}
