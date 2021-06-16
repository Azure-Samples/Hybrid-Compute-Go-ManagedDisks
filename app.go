package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	hybridresources "Hybrid-Compute-Go-ManagedDisks/hybridResources"
	hybridstorage "Hybrid-Compute-Go-ManagedDisks/hybridStorage"
	"Hybrid-Compute-Go-ManagedDisks/hybridcompute"
	"Hybrid-Compute-Go-ManagedDisks/hybridnetwork"

	"github.com/Azure/go-autorest/autorest/azure"
)

var (
	armEndpoint    = os.Getenv("AZURE_ARM_ENDPOINT")
	tenantID       = os.Getenv("AZURE_TENANT_ID")
	clientID       = os.Getenv("AZURE_SP_CERT_ID")
	certPass       = os.Getenv("AZURE_SP_CERT_PASS")
	certPath       = os.Getenv("AZURE_SP_CERT_PATH")
	subscriptionID = os.Getenv("AZURE_SUBSCRIPTION_ID")
	location       = os.Getenv("AZURE_LOCATION")

	vmName             = "az-samples-go-vmname"
	nicName            = "nic1"
	username           = "VMAdmin"
	virtualNetworkName = "vnet1"
	subnetName         = "subnet1"
	nsgName            = "nsg1"
	ipName             = "ip1"
	storageAccountName = strings.ToLower("samplestacc")
	rgName             = "azure-sample-rg"
	diskName           = "sampledisk"
)

func main() {
	// Password is not required when using SSH key pair.
	var password string
	if len(os.Args) == 2 {
		password = os.Args[1]
	} else if len(os.Args) > 2 {
		log.Fatalf("Error, invalid number of CLI arguments: %d", len(os.Args))
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Could not find user home directory. The sample code looks for .ssh folder in the user home directory %s.", homeDir)
	}
	sshPublicKeyPath := homeDir + filepath.FromSlash("/.ssh/id_rsa.pub")
	_, sshPubFileErr := os.Stat(sshPublicKeyPath)
	if sshPubFileErr != nil && len(os.Args) == 1 {
		log.Fatalf("Both VM admin password and SSH key pair path %s are invalid. At least one required to create VM. Usage for password authentication: go run app.go <PASSWORD>", sshPublicKeyPath)
	}
	cntx := context.Background()
	environment, _ := azure.EnvironmentFromURL(armEndpoint)
	splitEndpoint := strings.Split(environment.ActiveDirectoryEndpoint, "/")
	splitEndpointlastIndex := len(splitEndpoint) - 1
	if splitEndpoint[splitEndpointlastIndex] == "adfs" || splitEndpoint[splitEndpointlastIndex] == "adfs/" {
		tenantID = "adfs"
	}
	//Create a resource group on Azure Stack
	_, errRgStack := hybridresources.CreateResourceGroup(
		cntx,
		rgName,
		location,
		certPath,
		armEndpoint,
		tenantID,
		clientID,
		certPass,
		subscriptionID)
	if errRgStack != nil {
		fmt.Println(errRgStack.Error())
		return
	}

	// Create virtual network on Azure Stack
	_, errVnet := hybridnetwork.CreateVirtualNetworkAndSubnets(
		cntx,
		virtualNetworkName,
		subnetName,
		certPath,
		tenantID,
		clientID,
		certPass,
		armEndpoint,
		subscriptionID,
		rgName,
		location)
	if errVnet != nil {
		fmt.Println(errVnet.Error())
		return
	}

	//Create network security group on Azure Stack
	_, errSg := hybridnetwork.CreateNetworkSecurityGroup(
		cntx,
		nsgName,
		certPath,
		tenantID,
		clientID,
		certPass,
		armEndpoint,
		subscriptionID,
		rgName,
		location)
	if errSg != nil {
		fmt.Println(errSg.Error())
		return
	}

	// Create public IP on Azure Stack
	_, errIP := hybridnetwork.CreatePublicIP(
		cntx,
		ipName,
		certPath,
		tenantID,
		clientID,
		certPass,
		armEndpoint,
		subscriptionID,
		rgName,
		location)
	if errIP != nil {
		fmt.Println(errIP.Error())
	}

	// Create network interface on Azure Stack
	_, errNic := hybridnetwork.CreateNetworkInterface(
		cntx,
		nicName,
		nsgName,
		virtualNetworkName,
		subnetName,
		ipName,
		certPath,
		tenantID,
		clientID,
		certPass,
		armEndpoint,
		subscriptionID,
		rgName,
		location)
	if errNic != nil {
		fmt.Println(errNic.Error())
	}

	// Create storage account and disk on Azure Stack
	_, errSa := hybridstorage.CreateStorageAccount(
		cntx,
		storageAccountName,
		rgName,
		location,
		certPath,
		tenantID,
		clientID,
		certPass,
		armEndpoint,
		subscriptionID)
	if errSa != nil {
		fmt.Println(errSa.Error())
	}

	// Create virtual machine on Azure Stack
	_, errVM := hybridcompute.CreateVM(cntx,
		vmName,
		diskName,
		nicName,
		username,
		password,
		storageAccountName,
		sshPublicKeyPath,
		rgName,
		location,
		tenantID,
		clientID,
		certPass,
		certPath,
		armEndpoint,
		subscriptionID)
	if errVM != nil {
		fmt.Println(errVM.Error())
	}
	return
}
