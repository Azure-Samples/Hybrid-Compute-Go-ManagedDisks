// Copyright (c) Microsoft and contributors.  All rights reserved.
//
// This source code is licensed under the MIT license found in the
// LICENSE file in the root directory of this source tree.

package hybridcompute

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"../hybridnetwork"
	"../iam"

	"github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/compute/mgmt/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/to"
)

const (
	publisher   = "Canonical"
	offer       = "UbuntuServer"
	sku         = "16.04-LTS"
	errorPrefix = "Cannot create VM, reason: %v"
)

// fakepubkey is used if a key isn't available at the specified path in CreateVM(...)
var fakepubkey = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC7laRyN4B3YZmVrDEZLZoIuUA72pQ0DpGuZBZWykCofIfCPrFZAJgFvonKGgKJl6FGKIunkZL9Us/mV4ZPkZhBlE7uX83AAf5i9Q8FmKpotzmaxN10/1mcnEE7pFvLoSkwqrQSkrrgSm8zaJ3g91giXSbtqvSIj/vk2f05stYmLfhAwNo3Oh27ugCakCoVeuCrZkvHMaJgcYrIGCuFo6q0Pfk9rsZyriIqEa9AtiUOtViInVYdby7y71wcbl0AbbCZsTSqnSoVxm2tRkOsXV6+8X4SnwcmZbao3H+zfO1GBhQOLxJ4NQbzAa8IJh810rYARNLptgmsd4cYXVOSosTX azureuser"

func getVMClient(certPath, tenantID, clientID, clientSecret, armEndpoint, subscriptionID string) compute.VirtualMachinesClient {
	token, err := iam.GetResourceManagementToken(tenantID, clientID, clientSecret, armEndpoint, certPath)
	if err != nil {
		log.Fatal(fmt.Sprintf(errorPrefix, fmt.Sprintf("Cannot generate token. Error details: %v.", err)))
	}
	vmClient := compute.NewVirtualMachinesClientWithBaseURI(armEndpoint, subscriptionID)
	vmClient.Authorizer = autorest.NewBearerAuthorizer(token)
	return vmClient
}

func getDiskClient(certPath, tenantID, clientID, clientSecret, armEndpoint, subscriptionID string) compute.DisksClient {
	token, err := iam.GetResourceManagementToken(tenantID, clientID, clientSecret, armEndpoint, certPath)
	if err != nil {
		log.Fatal(fmt.Sprintf(errorPrefix, fmt.Sprintf("Cannot generate token. Error details: %v.", err)))
	}
	diskClient := compute.NewDisksClientWithBaseURI(armEndpoint, subscriptionID)
	diskClient.Authorizer = autorest.NewBearerAuthorizer(token)
	return diskClient
}

// CreateVM creates a new virtual machine with the specified name using the specified network interface and storage account.
// Username, password, and sshPublicKeyPath determine logon credentials.
func CreateVM(ctx context.Context, vmName, diskName, nicName, username, password, storageAccountName, sshPublicKeyPath, rgName, location, tenantID, clientID, clientSecret, certPath, armEndpoint, subscriptionID string) (vm compute.VirtualMachine, err error) {
	cntx := context.Background()
	nic, _ := hybridnetwork.GetNic(cntx, nicName, certPath, tenantID, clientID, clientSecret, armEndpoint, subscriptionID, rgName)

	var sshKeyData string
	_, err = os.Stat(sshPublicKeyPath)
	if err == nil {
		sshBytes, err := ioutil.ReadFile(sshPublicKeyPath)
		if err != nil {
			log.Fatalf(fmt.Sprintf(errorPrefix, fmt.Sprintf("failed to read SSH key data: %v", err)))
		}
		sshKeyData = string(sshBytes)
	} else {
		sshKeyData = fakepubkey
	}
	vmClient := getVMClient(certPath, tenantID, clientID, clientSecret, armEndpoint, subscriptionID)
	diskClient := getDiskClient(certPath, tenantID, clientID, clientSecret, armEndpoint, subscriptionID)
	diskFuture, _ := diskClient.CreateOrUpdate(ctx, rgName, diskName, compute.Disk{
		Location: to.StringPtr(location),
		DiskProperties: &compute.DiskProperties{
			CreationData: &compute.CreationData{
				CreateOption: compute.Empty,
			},
			DiskSizeGB: to.Int32Ptr(1),
		},
	})
	err = diskFuture.WaitForCompletionRef(ctx, diskClient.Client)
	if err != nil {
		return vm, err
	}
	disk, _ := diskFuture.Result(diskClient)
	hardwareProfile := &compute.HardwareProfile{
		VMSize: compute.StandardA1,
	}
	storageProfile := &compute.StorageProfile{
		ImageReference: &compute.ImageReference{
			Publisher: to.StringPtr(publisher),
			Offer:     to.StringPtr(offer),
			Sku:       to.StringPtr(sku),
			Version:   to.StringPtr("latest"),
		},
		DataDisks: &[]compute.DataDisk{
			compute.DataDisk{
				CreateOption: compute.DiskCreateOptionTypesAttach,
				ManagedDisk: &compute.ManagedDiskParameters{
					StorageAccountType: compute.StandardLRS,
					ID:                 disk.ID,
				},
				Caching:    compute.CachingTypesReadOnly,
				DiskSizeGB: to.Int32Ptr(1),
				Lun:        to.Int32Ptr(1),
				Name:       to.StringPtr(diskName),
			},
		},
		OsDisk: &compute.OSDisk{
			Name:         to.StringPtr("osDisk"),
			CreateOption: compute.DiskCreateOptionTypesFromImage,
		},
	}
	osProfile := &compute.OSProfile{
		ComputerName:  to.StringPtr(vmName),
		AdminUsername: to.StringPtr(username),
		AdminPassword: to.StringPtr(password),
		LinuxConfiguration: &compute.LinuxConfiguration{
			SSH: &compute.SSHConfiguration{
				PublicKeys: &[]compute.SSHPublicKey{
					{
						Path:    to.StringPtr(fmt.Sprintf("/home/%s/.ssh/authorized_keys", username)),
						KeyData: to.StringPtr(sshKeyData),
					},
				},
			},
		},
	}
	networkProfile := &compute.NetworkProfile{
		NetworkInterfaces: &[]compute.NetworkInterfaceReference{
			{
				ID: nic.ID,
				NetworkInterfaceReferenceProperties: &compute.NetworkInterfaceReferenceProperties{
					Primary: to.BoolPtr(true),
				},
			},
		},
	}
	virtualMachine := compute.VirtualMachine{
		Location: to.StringPtr(location),
		VirtualMachineProperties: &compute.VirtualMachineProperties{
			HardwareProfile: hardwareProfile,
			StorageProfile:  storageProfile,
			OsProfile:       osProfile,
			NetworkProfile:  networkProfile,
		},
	}
	future, err := vmClient.CreateOrUpdate(
		cntx,
		rgName,
		vmName,
		virtualMachine,
	)
	if err != nil {
		return vm, fmt.Errorf(fmt.Sprintf(errorPrefix, err))
	}
	err = future.WaitForCompletionRef(cntx, vmClient.Client)
	if err != nil {
		return vm, fmt.Errorf(fmt.Sprintf(errorPrefix, err))
	}
	return future.Result(vmClient)
}
