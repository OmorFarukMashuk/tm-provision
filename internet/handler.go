package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"

	//"strings"

	"telmax-provision/structs"
	//"time"
)

func HandleProvision(request telmaxprovision.ProvisionRequest) {
	log.Infof("Got provision request %v", request)
	switch request.RequestType {
	case "New":
		NewRequest(request)

	case "Update":
		//		NewRequest(request)

	case "DeviceReturn":

	case "Cancel":
	}

}

func NewRequest(request telmaxprovision.ProvisionRequest) {
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}

	subscribe, err := telmax.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("Problem getting subscriber %v", err)
	}
	if subscribe.NetworkType == "Fibre" {
		subscriber := subscribe.AccountCode + "-" + subscribe.SubscribeCode

		// Check to see if they have any Internet services
		pools := map[string]bool{}
		for _, product := range request.Products {
			product := telmax.GetProduct(CoreDB, "product_code", product.ProductCode)
			if product.NetworkProfile != nil {
				profile := *product.NetworkProfile
				pools[profile.AddressPool] = true
			}
		}
		
		// If yes, then assign an IP address in DHCP
		for pool, _ := range pools {
			lease := 
		}

		// Get the ONT information
		for _, device := range request.Devices {
			if device.DeviceType == "AccessEndpoint" {
				var definition telmax.DeviceDefinition
				definition, err = telmax.GetDeviceDefinition(CoreDB, "devicedefinition_code", device.DefinitionCode)
				if definition.Vendor == "AdTran" && definition.Upstream == "XGS-PON" {

				}

			}
		}
		// Create ONT record

		// Create ONT interfaces

		// Add services

	} else {
		log.Info("Not a fibre customer")
	}
	return
}
