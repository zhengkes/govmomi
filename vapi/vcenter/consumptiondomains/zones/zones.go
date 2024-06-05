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

package zones

import (
	"context"
	"net/http"

	"github.com/zhengkes/govmomi/vapi/rest"
)

const (
	basePath = "/api/vcenter/consumption-domains/zones"
)

// Manager extends rest.Client, adding vSphere Zone related methods.
type Manager struct {
	*rest.Client
}

// NewManager creates a new Manager instance with the given client.
func NewManager(client *rest.Client) *Manager {
	return &Manager{
		Client: client,
	}
}

// CreateSpec
// https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/data-structures/ConsumptionDomains_Zones_CreateSpec
type CreateSpec struct {
	Zone        string `json:"zone"`
	Description string `json:"description"`
}

// ZoneInfo
// https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/data-structures/ConsumptionDomains_Zones_Info
type ZoneInfo struct {
	Description string `json:"description"`
}

// ListItem
// https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/data-structures/ConsumptionDomains_Zones_ListItem
type ListItem struct {
	Zone string   `json:"zone"`
	Info ZoneInfo `json:"info"`
}

// ListResult
// https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/data-structures/ConsumptionDomains_Zones_ListResult
type ListResult struct {
	Items []ListItem `json:"items"`
}

// CreateZone creates a vSphere Zone. Returns the identifier of the new zone and an error if the operation fails
// https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/api/vcenter/consumption-domains/zones/post
func (c *Manager) CreateZone(spec CreateSpec) (string, error) {
	path := c.Resource(basePath)
	req := path.Request(http.MethodPost, spec)
	var res string
	return res, c.Do(context.Background(), req, &res)
}

// DeleteZone deletes a vSphere Zone
// https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/api/vcenter/consumption-domains/zones/zone/delete
func (c *Manager) DeleteZone(zone string) error {
	path := c.Resource(basePath).WithSubpath(zone)
	req := path.Request(http.MethodDelete)
	return c.Do(context.Background(), req, nil)
}

// GetZone returns the details of a vSphere Zone
// https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/api/vcenter/consumption-domains/zones/zone/get
func (c *Manager) GetZone(zone string) (ZoneInfo, error) {
	path := c.Resource(basePath).WithSubpath(zone)
	req := path.Request(http.MethodGet)
	var res ZoneInfo
	return res, c.Do(context.Background(), req, &res)
}

// ListZones returns the details of all vSphere Zones
// https://developer.broadcom.com/xapis/vsphere-automation-api/latest/vcenter/api/vcenter/consumption-domains/zones/get
func (c *Manager) ListZones() ([]ListItem, error) {
	path := c.Resource(basePath)
	req := path.Request(http.MethodGet)
	var res ListResult

	if err := c.Do(context.Background(), req, &res); err != nil {
		return nil, err
	}

	return res.Items, nil
}
