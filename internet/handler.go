package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"

	//"strings"
	"bitbucket.org/timstpierre/telmax-common"
	"bitbucket.org/timstpierre/telmax-provision/dhcpdb"
	"bitbucket.org/timstpierre/telmax-provision/kafka"
	"bitbucket.org/timstpierre/telmax-provision/structs"
	"strconv"
	"time"
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
		if subscribe.Wirecentre == "" {
			result.Result = "Subscriber wirecentre not set - mandatory!"
			kafka.SubmitResult(result)
			return
		}

		// Check to see if they have any Internet services
		pools := map[string]bool{}
		reservations := map[string]dhcpdb.Reservation{}
		for _, product := range request.Products {
			var productData telmax.Product
			productData, err = telmax.GetProduct(CoreDB, "product_code", product.ProductCode)
			if productData.NetworkProfile != nil {
				profile := *productData.NetworkProfile
				pools[profile.AddressPool] = true
			}
		}

		// If yes, then assign an IP address in DHCP
		for pool, _ := range pools {
			reservations[pool], err = dhcpdb.DhcpAssign(subscribe.Wirecentre, pool, subscriber)
			var resulttext string
			if err != nil {
				resulttext = resulttext + "Problem assigning address " + err.Error() + "\n"
			} else {
				resulttext = resulttext + "Assigned address from pool " + pool + " vlan " + strconv.Itoa(reservations[pool].VlanID) + "\n"
				result.Success = true
			}
			result.Result = resulttext
			kafka.SubmitResult(result)
		}

		// Get the ONT information
		var activeONT struct {
			Device     telmax.Device
			Definition telmax.DeviceDefinition
		}
		var hasONT bool
		for _, device := range request.Devices {
			if device.DeviceType == "AccessEndpoint" {
				var definition telmax.DeviceDefinition
				definition, err = telmax.GetDeviceDefinition(CoreDB, "devicedefinition_code", device.DefinitionCode)
				if definition.Vendor == "AdTran" && definition.Upstream == "XGS-PON" {
					log.Infof("Found ONT")
					activeONT.Definition = definition
					activeONT.Device, err = telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
					hasONT = true
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
