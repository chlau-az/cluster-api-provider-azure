/*
Copyright 2019 The Kubernetes Authors.

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

package virtualnetworks

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	infrav1 "github.com/chlau-az/cluster-api-provider-azure/api/v1alpha3"
	azure "github.com/chlau-az/cluster-api-provider-azure/cloud"
	"github.com/chlau-az/cluster-api-provider-azure/cloud/converters"
)

// Spec input specification for Get/CreateOrUpdate/Delete calls
type Spec struct {
	ResourceGroup string
	Name          string
	CIDR          string
}

// getExisting provides information about an existing virtual network.
func (s *Service) getExisting(ctx context.Context, spec *Spec) (*infrav1.VnetSpec, error) {
	vnet, err := s.Client.Get(ctx, spec.ResourceGroup, spec.Name)
	if err != nil {
		if azure.ResourceNotFound(err) {
			return nil, err
		}
		return nil, errors.Wrapf(err, "failed to get VNet %s", spec.Name)
	}
	cidr := ""
	if vnet.VirtualNetworkPropertiesFormat != nil && vnet.VirtualNetworkPropertiesFormat.AddressSpace != nil {
		prefixes := to.StringSlice(vnet.VirtualNetworkPropertiesFormat.AddressSpace.AddressPrefixes)
		if prefixes != nil && len(prefixes) > 0 {
			cidr = prefixes[0]
		}
	}
	return &infrav1.VnetSpec{
		ResourceGroup: spec.ResourceGroup,
		ID:            to.String(vnet.ID),
		Name:          to.String(vnet.Name),
		CidrBlock:     cidr,
		Tags:          converters.MapToTags(vnet.Tags),
	}, nil
}

// Reconcile gets/creates/updates a virtual network.
func (s *Service) Reconcile(ctx context.Context, spec interface{}) error {
	// Following should be created upstream and provided as an input to NewService
	// A VNet has following dependencies
	//    * VNet Cidr
	//    * Control Plane Subnet Cidr
	//    * Node Subnet Cidr
	//    * Control Plane NSG
	//    * Node NSG
	//    * Node Route Table
	vnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid VNET Specification")
	}

	existingVnet, err := s.getExisting(ctx, vnetSpec)
	if !azure.ResourceNotFound(err) {
		if err != nil {
			return errors.Wrap(err, "failed to get VNet")
		}

		if !existingVnet.IsManaged(s.Scope.ClusterName()) {
			s.Scope.V(2).Info("Working on custom VNet", "vnet-id", existingVnet.ID)
		}
		// vnet already exists, cannot update since it's immutable
		existingVnet.DeepCopyInto(s.Scope.Vnet())
		return nil
	}
	klog.V(2).Infof("creating VNet %s ", vnetSpec.Name)
	vnetProperties := network.VirtualNetwork{
		Tags: converters.TagsToMap(infrav1.Build(infrav1.BuildParams{
			ClusterName: s.Scope.ClusterName(),
			Lifecycle:   infrav1.ResourceLifecycleOwned,
			Name:        to.StringPtr(vnetSpec.Name),
			Role:        to.StringPtr(infrav1.CommonRole),
			Additional:  s.Scope.AdditionalTags(),
		})),
		Location: to.StringPtr(s.Scope.Location()),
		VirtualNetworkPropertiesFormat: &network.VirtualNetworkPropertiesFormat{
			AddressSpace: &network.AddressSpace{
				AddressPrefixes: &[]string{vnetSpec.CIDR},
			},
		},
	}
	err = s.Client.CreateOrUpdate(ctx, vnetSpec.ResourceGroup, vnetSpec.Name, vnetProperties)
	if err != nil {
		return err
	}

	klog.V(2).Infof("successfully created VNet %s ", vnetSpec.Name)
	return nil
}

// Delete deletes the virtual network with the provided name.
func (s *Service) Delete(ctx context.Context, spec interface{}) error {
	if !s.Scope.Vnet().IsManaged(s.Scope.ClusterName()) {
		s.Scope.V(4).Info("Skipping VNet deletion in custom vnet mode")
		return nil
	}
	vnetSpec, ok := spec.(*Spec)
	if !ok {
		return errors.New("Invalid VNET Specification")
	}
	klog.V(2).Infof("deleting VNet %s ", vnetSpec.Name)
	err := s.Client.Delete(ctx, vnetSpec.ResourceGroup, vnetSpec.Name)
	if err != nil && azure.ResourceNotFound(err) {
		// already deleted
		return nil
	}
	if err != nil {
		return errors.Wrapf(err, "failed to delete VNet %s in resource group %s", vnetSpec.Name, vnetSpec.ResourceGroup)
	}

	klog.V(2).Infof("successfully deleted VNet %s ", vnetSpec.Name)
	return nil
}
