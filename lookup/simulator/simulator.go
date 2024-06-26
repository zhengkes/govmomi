/*
Copyright (c) 2018 VMware, Inc. All Rights Reserved.

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
	"net/url"
	"strings"
	"sync"

	"github.com/zhengkes/govmomi/lookup"
	"github.com/zhengkes/govmomi/lookup/methods"
	"github.com/zhengkes/govmomi/lookup/types"
	"github.com/zhengkes/govmomi/simulator"
	"github.com/zhengkes/govmomi/vim25/soap"
	vim "github.com/zhengkes/govmomi/vim25/types"
)

var content = types.LookupServiceContent{
	LookupService:                vim.ManagedObjectReference{Type: "LookupLookupService", Value: "lookupService"},
	ServiceRegistration:          &vim.ManagedObjectReference{Type: "LookupServiceRegistration", Value: "ServiceRegistration"},
	DeploymentInformationService: vim.ManagedObjectReference{Type: "LookupDeploymentInformationService", Value: "deploymentInformationService"},
	L10n:                         vim.ManagedObjectReference{Type: "LookupL10n", Value: "l10n"},
}

func init() {
	simulator.RegisterEndpoint(func(s *simulator.Service, r *simulator.Registry) {
		if r.IsVPX() {
			s.RegisterSDK(New())
		}
	})
}

func New() *simulator.Registry {
	r := simulator.NewRegistry()
	r.Namespace = lookup.Namespace
	r.Path = lookup.Path

	r.Put(&ServiceInstance{
		ManagedObjectReference: lookup.ServiceInstance,
		Content:                content,
		register: func() {
			r.Put(&ServiceRegistration{
				ManagedObjectReference: *content.ServiceRegistration,
				Info:                   registrationInfo(),
			})
		},
	})

	return r
}

type ServiceInstance struct {
	vim.ManagedObjectReference

	Content types.LookupServiceContent

	instance sync.Once
	register func()
}

func (s *ServiceInstance) RetrieveServiceContent(_ *types.RetrieveServiceContent) soap.HasFault {
	// defer register to this point to ensure we can include vcsim's cert in ServiceEndpoints.SslTrust
	// TODO: we should be able to register within New(), but this is the only place that currently depends on vcsim's cert.
	s.instance.Do(s.register)

	return &methods.RetrieveServiceContentBody{
		Res: &types.RetrieveServiceContentResponse{
			Returnval: s.Content,
		},
	}
}

type ServiceRegistration struct {
	vim.ManagedObjectReference

	Info []types.LookupServiceRegistrationInfo
}

func (s *ServiceRegistration) GetSiteId(_ *types.GetSiteId) soap.HasFault {
	return &methods.GetSiteIdBody{
		Res: &types.GetSiteIdResponse{
			Returnval: siteID,
		},
	}
}

func matchServiceType(filter, info *types.LookupServiceRegistrationServiceType) bool {
	if filter.Product != "" {
		if filter.Product != info.Product {
			return false
		}
	}

	if filter.Type != "" {
		if filter.Type != info.Type {
			return false
		}
	}

	return true
}

func matchEndpointType(filter, info *types.LookupServiceRegistrationEndpointType) bool {
	if filter.Protocol != "" {
		if filter.Protocol != info.Protocol {
			return false
		}
	}

	if filter.Type != "" {
		if filter.Type != info.Type {
			return false
		}
	}

	return true
}

func (s *ServiceRegistration) List(req *types.List) soap.HasFault {
	body := new(methods.ListBody)
	filter := req.FilterCriteria

	if filter == nil {
		// This is what a real PSC returns if FilterCriteria is nil.
		body.Fault_ = simulator.Fault("LookupFaultServiceFault", &vim.SystemError{
			Reason: "Invalid fault",
		})
		return body
	}
	body.Res = new(types.ListResponse)

	for _, info := range s.Info {
		if filter.SiteId != "" {
			if filter.SiteId != info.SiteId {
				continue
			}
		}
		if filter.NodeId != "" {
			if filter.NodeId != info.NodeId {
				continue
			}
		}
		if filter.ServiceType != nil {
			if !matchServiceType(filter.ServiceType, &info.ServiceType) {
				continue
			}
		}
		if filter.EndpointType != nil {
			services := info.ServiceEndpoints
			info.ServiceEndpoints = nil
			for _, service := range services {
				if !matchEndpointType(filter.EndpointType, &service.EndpointType) {
					continue
				}
				info.ServiceEndpoints = append(info.ServiceEndpoints, service)
			}
			if len(info.ServiceEndpoints) == 0 {
				continue
			}
		}
		body.Res.Returnval = append(body.Res.Returnval, info)
	}

	return body
}

// BreakLookupServiceURLs makes the path of all lookup service urls invalid
func BreakLookupServiceURLs() {
	setting := simulator.Map.OptionManager().Setting

	for _, s := range setting {
		o := s.GetOptionValue()
		if strings.HasSuffix(o.Key, ".uri") {
			val := o.Value.(string)
			u, _ := url.Parse(val)
			u.Path = "/enoent" + u.Path
			o.Value = u.String()
		}
	}
}
