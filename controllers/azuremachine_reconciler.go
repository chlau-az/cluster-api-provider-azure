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

package controllers

import (
	"context"
	"encoding/base64"

	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/inboundnatrules"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/roleassignments"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"k8s.io/klog"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/disks"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/networkinterfaces"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/publicips"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/virtualmachines"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
)

// azureMachineService is the group of services called by the AzureMachine controller
type azureMachineService struct {
	machineScope         *scope.MachineScope
	clusterScope         *scope.ClusterScope
	networkInterfacesSvc azure.Service
	inboundNatRulesSvc   azure.Service
	virtualMachinesSvc   *virtualmachines.Service
	roleAssignmentsSvc   azure.Service
	disksSvc             azure.Service
	publicIPsSvc         azure.Service
	skuCache             *resourceskus.Cache
}

// newAzureMachineService populates all the services based on input scope
func newAzureMachineService(machineScope *scope.MachineScope, clusterScope *scope.ClusterScope) *azureMachineService {
	cache := resourceskus.NewCache(clusterScope, clusterScope.Location())

	return &azureMachineService{
		machineScope:         machineScope,
		clusterScope:         clusterScope,
		inboundNatRulesSvc:   inboundnatrules.NewService(machineScope),
		networkInterfacesSvc: networkinterfaces.NewService(machineScope, cache),
		virtualMachinesSvc:   virtualmachines.NewService(clusterScope, machineScope, cache),
		roleAssignmentsSvc:   roleassignments.NewService(machineScope),
		disksSvc:             disks.NewService(machineScope),
		publicIPsSvc:         publicips.NewService(machineScope),
		skuCache:             cache,
	}
}

// Reconcile reconciles all the services in pre determined order
func (s *azureMachineService) Reconcile(ctx context.Context) (*infrav1.VM, error) {
	err := s.publicIPsSvc.Reconcile(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create public IPs")
	}

	err = s.inboundNatRulesSvc.Reconcile(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create inbound NAT rule")
	}

	err = s.networkInterfacesSvc.Reconcile(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create VM network interface")
	}

	vm, vmErr := s.reconcileVirtualMachine(ctx, azure.GenerateNICName(s.machineScope.Name()))
	if vmErr != nil {
		return nil, errors.Wrapf(vmErr, "failed to create VM %s ", s.machineScope.Name())
	}

	err = s.roleAssignmentsSvc.Reconcile(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create role assignment")
	}

	return vm, nil
}

// Delete deletes all the services in pre determined order
func (s *azureMachineService) Delete(ctx context.Context) error {
	vmSpec := &virtualmachines.Spec{
		Name: s.machineScope.Name(),
	}

	err := s.virtualMachinesSvc.Delete(ctx, vmSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to delete machine")
	}

	err = s.networkInterfacesSvc.Delete(ctx)
	if err != nil {
		return errors.Wrapf(err, "Unable to delete network interface")
	}

	err = s.inboundNatRulesSvc.Delete(ctx)
	if err != nil {
		return errors.Wrapf(err, "Unable to delete inbound NAT rule")
	}

	err = s.publicIPsSvc.Delete(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to delete public IPs")
	}

	err = s.disksSvc.Delete(ctx)
	if err != nil {
		return errors.Wrapf(err, "Failed to delete OS disk of machine %s", s.machineScope.Name())
	}

	return nil
}

func (s *azureMachineService) VMIfExists(ctx context.Context, id *string) (*infrav1.VM, error) {
	if id == nil {
		s.clusterScope.Info("VM does not have an ID")
		s.clusterScope.Info("VM does not have an ID")
		return nil, nil
	}

	vmSpec := &virtualmachines.Spec{
		Name: s.machineScope.Name(),
	}
	vm, err := s.virtualMachinesSvc.Get(ctx, vmSpec)

	if azure.ResourceNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, errors.Wrap(err, "Failed to get VM")
	}

	klog.V(2).Infof("Found VM for AzureMachine %s", s.machineScope.Name())

	return vm, nil
}

// getVirtualMachineZone gets a random availability zones from available set,
// this will hopefully be an input from upstream machinesets so all the vms are balanced
func (s *azureMachineService) getVirtualMachineZone(ctx context.Context) (string, error) {
	vmName := s.machineScope.AzureMachine.Name
	vmSize := s.machineScope.AzureMachine.Spec.VMSize
	location := s.machineScope.AzureMachine.Spec.Location

	zones, err := s.skuCache.GetZonesWithVMSize(ctx, vmSize, location)
	if err != nil {
		return "", errors.Wrapf(err, "failed to get zones for VM size %s", vmSize)
	}

	if len(zones) <= 0 {
		return "", nil
	}

	zone := s.machineScope.AvailabilityZone()
	var selectedZone string

	// DEPRECATED: to support old clients
	if zone == "" && s.machineScope.AzureMachine.Spec.AvailabilityZone.ID != nil {
		zone = *s.machineScope.AzureMachine.Spec.AvailabilityZone.ID
	}

	if zone != "" {
		for _, allowedZone := range zones {
			if allowedZone == zone {
				selectedZone = zone
				break
			}
		}
	} else {
		klog.Infof("Selecting first available AZ as no availability zone was set or user-provided availability zone is not supported for VM size %s in location %s", vmSize, location)
		selectedZone = zones[0]
	}

	klog.Infof("Selected availability zone %s for %s", selectedZone, vmName)

	return selectedZone, nil
}

func (s *azureMachineService) reconcileVirtualMachine(ctx context.Context, nicName string) (*infrav1.VM, error) {
	decoded, err := base64.StdEncoding.DecodeString(s.machineScope.AzureMachine.Spec.SSHPublicKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode ssh public key")
	}

	var vmZone string
	azSupported := s.isAvailabilityZoneSupported()
	if azSupported {
		useAZ := true

		if s.machineScope.AzureMachine.Spec.AvailabilityZone.Enabled != nil {
			useAZ = *s.machineScope.AzureMachine.Spec.AvailabilityZone.Enabled
		}

		if useAZ {
			var zoneErr error
			vmZone, zoneErr = s.getVirtualMachineZone(ctx)
			if zoneErr != nil {
				return nil, errors.Wrap(zoneErr, "failed to get availability zone")
			}
		}
	}

	nicNames := []string{nicName}
	if s.machineScope.AzureMachine.Spec.AllocatePublicIP == true {
		nicNames = append(nicNames, azure.GeneratePublicNICName(s.machineScope.Name()))
	}

	image, err := getVMImage(s.machineScope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get VM image")
	}

	bootstrapData, err := s.machineScope.GetBootstrapData(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve bootstrap data")
	}

	vmSpec := &virtualmachines.Spec{
		Name:                   s.machineScope.Name(),
		NICNames:               nicNames,
		SSHKeyData:             string(decoded),
		Size:                   s.machineScope.AzureMachine.Spec.VMSize,
		OSDisk:                 s.machineScope.AzureMachine.Spec.OSDisk,
		DataDisks:              s.machineScope.AzureMachine.Spec.DataDisks,
		Image:                  image,
		CustomData:             bootstrapData,
		Zone:                   vmZone,
		Identity:               s.machineScope.AzureMachine.Spec.Identity,
		UserAssignedIdentities: s.machineScope.AzureMachine.Spec.UserAssignedIdentities,
		SpotVMOptions:          s.machineScope.AzureMachine.Spec.SpotVMOptions,
	}

	err = s.virtualMachinesSvc.Reconcile(ctx, vmSpec)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to reconcile virtual machine")
	}

	newVM, err := s.virtualMachinesSvc.Get(ctx, vmSpec)
	if err != nil {
		return newVM, errors.Wrapf(err, "failed to get VM %s in %s", vmSpec.Name, s.clusterScope.ResourceGroup())
	}
	if newVM != nil {
		if newVM.State == infrav1.VMStateFailed {
			// If VM failed provisioning, delete it so it can be recreated
			err = s.virtualMachinesSvc.Delete(ctx, vmSpec)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to delete machine")
			}

			err = s.disksSvc.Delete(ctx)
			if err != nil && !azure.ResourceNotFound(err) {
				return nil, errors.Wrapf(err, "failed to delete OS disk of machine %s", s.machineScope.Name())
			}
			return nil, errors.Errorf("virtual machine %s is deleted, retry creating in next reconcile", s.machineScope.Name())
		} else if newVM.State != infrav1.VMStateSucceeded {
			return nil, errors.Errorf("virtual machine %s is still in provisioning state %s, reconcile", s.machineScope.Name(), newVM.State)
		}
	}
	return newVM, nil
}

// GetControlPlaneMachines retrieves all non-deleted control plane nodes from a MachineList
func GetControlPlaneMachines(machineList *clusterv1.MachineList) []*clusterv1.Machine {
	var cpm []*clusterv1.Machine
	for _, m := range machineList.Items {
		m := m
		if util.IsControlPlaneMachine(&m) {
			cpm = append(cpm, m.DeepCopy())
		}
	}
	return cpm
}

// isAvailabilityZoneSupported determines if Availability Zones are supported in a selected location
// based on SupportedAvailabilityZoneLocations. Returns true if supported.
func (s *azureMachineService) isAvailabilityZoneSupported() bool {
	azSupported := false

	for _, supportedLocation := range azure.SupportedAvailabilityZoneLocations {
		if s.machineScope.Location() == supportedLocation {
			azSupported = true

			return azSupported
		}
	}

	s.machineScope.V(2).Info("Availability Zones are not supported in the selected location", "location", s.machineScope.Location())
	return azSupported
}

// Pick image from the machine configuration, or use a default one.
func getVMImage(scope *scope.MachineScope) (*infrav1.Image, error) {
	// Use custom Marketplace image, Image ID or a Shared Image Gallery image if provided
	if scope.AzureMachine.Spec.Image != nil {
		return scope.AzureMachine.Spec.Image, nil
	}
	scope.Info("No image specified for machine, using default", "machine", scope.AzureMachine.GetName())
	return azure.GetDefaultUbuntuImage(to.String(scope.Machine.Spec.Version))
}
