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

package mo

import (
	"github.com/zhengkes/govmomi/eam/types"
	vim "github.com/zhengkes/govmomi/vim25/types"
)

// Agency handles the deployment of a single type of agent virtual
// machine and any associated VIB bundle, on a set of compute resources.
type Agency struct {
	EamObject `yaml:",inline"`

	Agent      []vim.ManagedObjectReference `json:"agent,omitempty"`
	Config     types.BaseAgencyConfigInfo   `json:"config"`
	Runtime    types.EamObjectRuntimeInfo   `json:"runtime"`
	SolutionId string                       `json:"solutionId,omitempty"`
}
