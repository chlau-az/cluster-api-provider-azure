/*
Copyright 2020 The Kubernetes Authors.

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

package resourceskus

import (
	"context"
	"strings"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-12-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/pkg/errors"

	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	List(context.Context, string) ([]compute.ResourceSku, error)
	HasAcceleratedNetworking(context.Context, string) (bool, error)
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	skus compute.ResourceSkusClient
}

var _ Client = &AzureClient{}

// NewClient creates a new Resource SKUs client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	return &AzureClient{
		skus: newResourceSkusClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer()),
	}
}

// newResourceSkusClient creates a new Resource SKUs client from subscription ID.
func newResourceSkusClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.ResourceSkusClient {
	c := compute.NewResourceSkusClientWithBaseURI(baseURI, subscriptionID)
	c.Authorizer = authorizer
	_ = c.AddToUserAgent(azure.UserAgent()) // intentionally ignore error as it doesn't matter
	return c
}

// List returns all Resource SKUs available to the subscription.
func (ac *AzureClient) List(ctx context.Context, filter string) ([]compute.ResourceSku, error) {
	iter, err := ac.skus.ListComplete(ctx, filter)
	if err != nil {
		return nil, errors.Wrap(err, "could not list resource skus")
	}

	var skus []compute.ResourceSku
	for iter.NotDone() {
		skus = append(skus, iter.Value())
		if err := iter.NextWithContext(ctx); err != nil {
			return skus, errors.Wrap(err, "could not iterate resource skus")
		}
	}

	return skus, nil
}

// HasAcceleratedNetworking returns whether the given compute SKU supports accelerated networking.
func (ac *AzureClient) HasAcceleratedNetworking(ctx context.Context, name string) (bool, error) {
	if name == "" {
		return false, nil
	}
	skus, err := ac.List(ctx, "") // "filter" argument only works for location, so filter in code
	if err != nil {
		return false, err
	}
	for _, sku := range skus {
		if sku.Name != nil && *sku.Name == name {
			if sku.Capabilities != nil {
				for _, c := range *sku.Capabilities {
					if c.Name != nil && *c.Name == "AcceleratedNetworkingEnabled" {
						if c.Value != nil && strings.EqualFold(*c.Value, "True") {
							return true, nil
						}
					}
				}
			}
			break
		}
	}
	return false, nil
}
