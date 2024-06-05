/*
Copyright (c) 2022 VMware, Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0.
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package consolecli

import (
	"context"
	"net/http"

	"github.com/zhengkes/govmomi/vapi/rest"
)

const Path = "/api/appliance/access/consolecli"

// Manager provides convenience methods to get/set enabled state of CLI.
type Manager struct {
	*rest.Client
}

// NewManager creates a new Manager with the given client
func NewManager(client *rest.Client) *Manager {
	return &Manager{
		Client: client,
	}
}

// Get returns enabled state of the console-based controlled CLI (TTY1).
func (m *Manager) Get(ctx context.Context) (bool, error) {
	r := m.Resource(Path)

	var status bool
	return status, m.Do(ctx, r.Request(http.MethodGet), &status)
}

// Access represents the value to be set for ConsoleCLI
type Access struct {
	Enabled bool `json:"enabled"`
}

// Set enables state of the console-based controlled CLI (TTY1).
func (m *Manager) Set(ctx context.Context, inp Access) error {
	r := m.Resource(Path)

	return m.Do(ctx, r.Request(http.MethodPut, inp), nil)
}
