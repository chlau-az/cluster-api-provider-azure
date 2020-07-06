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

package availabilityzones

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest"
	azure "github.com/chlau-az/cluster-api-provider-azure/cloud"
)

// Client wraps go-sdk
type Client interface {
	ListComplete(context.Context) (compute.ResourceSkusResultIterator, error)
}

// AzureClient contains the Azure go-sdk Client
type AzureClient struct {
	resourceSkus compute.ResourceSkusClient
}

var _ Client = &AzureClient{}

// NewClient creates a new VM client from subscription ID.
func NewClient(auth azure.Authorizer) *AzureClient {
	c := newResourceSkusClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &AzureClient{c}
}

// getResourceSkusClient creates a new availability zones client from subscription ID.
func newResourceSkusClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) compute.ResourceSkusClient {
	skusClient := compute.NewResourceSkusClientWithBaseURI(baseURI, subscriptionID)
	skusClient.Authorizer = authorizer
	skusClient.AddToUserAgent(azure.UserAgent())
	return skusClient
}

// ListComplete enumerates all values, automatically crossing page boundaries as required.
func (ac *AzureClient) ListComplete(ctx context.Context) (compute.ResourceSkusResultIterator, error) {
	return ac.resourceSkus.ListComplete(ctx)
}
