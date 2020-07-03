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

package publicips

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/network/mgmt/network"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Reconcile gets/creates/updates a public ip.
func (s *Service) Reconcile(ctx context.Context) error {
	for _, ip := range s.Scope.PublicIPSpecs() {
		klog.V(2).Infof("creating public IP %s", ip.Name)

		err := s.Client.CreateOrUpdate(
			ctx,
			s.Scope.ResourceGroup(),
			ip.Name,
			network.PublicIPAddress{
				Sku:      &network.PublicIPAddressSku{Name: network.PublicIPAddressSkuNameStandard},
				Name:     to.StringPtr(ip.Name),
				Location: to.StringPtr(s.Scope.Location()),
				PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
					PublicIPAddressVersion:   network.IPv4,
					PublicIPAllocationMethod: network.Static,
					DNSSettings: &network.PublicIPAddressDNSSettings{
						DomainNameLabel: to.StringPtr(strings.ToLower(ip.Name)),
						Fqdn:            to.StringPtr(ip.DNSName),
					},
				},
			},
		)

		if err != nil {
			return errors.Wrap(err, "cannot create public IP")
		}
		klog.V(2).Infof("successfully created public IP %s", ip.Name)
	}
	return nil
}

// Delete deletes the public IP with the provided scope.
func (s *Service) Delete(ctx context.Context) error {
	for _, ip := range s.Scope.PublicIPSpecs() {
		klog.V(2).Infof("deleting public IP %s", ip.Name)
		err := s.Client.Delete(ctx, s.Scope.ResourceGroup(), ip.Name)
		if err != nil && azure.ResourceNotFound(err) {
			// already deleted
			return nil
		}
		if err != nil {
			return errors.Wrapf(err, "failed to delete public IP %s in resource group %s", ip.Name, s.Scope.ResourceGroup())
		}

		klog.V(2).Infof("deleted public IP %s", ip.Name)
	}
	return nil
}
