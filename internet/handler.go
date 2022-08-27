/*
	Internet provisioning handler for telMAX distributed provisioning system

	This service will consume provision requests and execute network provisioning to enable
	Internet services on FTTx networks using AdTran MCP.  This service also interacts with
	the circuit database to assign and maintain access circuit records, as well as the DHCP
	database to allocate IP address resources to customers.
*/
package main

import (
	"fmt"
	"strings"
	"time"

	"bitbucket.org/telmaxdc/telmax-common"
	"bitbucket.org/telmaxdc/telmax-common/devices"
	"bitbucket.org/telmaxdc/telmax-common/maxbill"
	"bitbucket.org/telmaxdc/telmax-provision/dhcpdb"
	"bitbucket.org/telmaxdc/telmax-provision/kafka"
	"bitbucket.org/telmaxdc/telmax-provision/mcp"
	"bitbucket.org/telmaxdc/telmax-provision/netdb"
	telmaxprovision "bitbucket.org/telmaxdc/telmax-provision/structs"

	log "github.com/sirupsen/logrus"
)

// Accept and process a provision request and invoke the appropriate functions based on the request type
func HandleProvision(request telmaxprovision.ProvisionRequest) {
	log.Infof("Got provision request %v", request)
	switch request.RequestType {
	case "New":
		NewRequest(request)

	case "Update":
		//		NewRequest(request)

	case "Device Swap":
		log.Info("Handling device swap request")
		DeviceSwap(request)

	case "DeviceReturn":

	case "UnProvision":
		UnProvisionServices(request)

	case "Cancel":
		UnProvisionServices(request)
		DeleteONT(request)
		//		ReleaseCircuit(request)

	}

}

// Provision services as new (check to see if they exist already)
func NewRequest(request telmaxprovision.ProvisionRequest) {
	var (
		site       Site
		subscriber string
		PON        string
	)
	// result is the Kafka response object
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
		Success:   false, // let a success set this bool
	}
	subscribe, err := maxbill.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("getting subscriber (%s)(%s) %v", request.AccountCode, request.SubscribeCode, err)
		result.Result = fmt.Sprintf("Problem getting subscriber (%s)(%s) %v", request.AccountCode, request.SubscribeCode, err)
		kafka.SubmitResult(result)
	}
	// We only act on this if the subscription is Fibre
	if subscribe.NetworkType != "Fibre" {
		log.Debugf("not a fibre customer")
		return
	}
	subscriber = subscribe.AccountCode + "-" + subscribe.SubscribeCode

	// Check to see if there is a site ID
	if subscribe.SiteID == "" {
		result.Result = "Subscriber site ID not set - mandatory!"
		kafka.SubmitResult(result)
		return
	}
	// extract the Site info
	site, err = GetSite(subscribe.SiteID)
	if err != nil {
		log.Errorf("getting site (%s) -  %v", subscribe.SiteID, err)
		result.Result = fmt.Sprintf("Problem getting site (%s) -  %v", subscribe.SiteID, err)
		kafka.SubmitResult(result)
		return
	}
	// validate the Circuit Data
	if len(site.CircuitData) < 1 {
		log.Errorf("site (%s) does not have valid circuit data", subscribe.SiteID)
		result.Result = "This site does not have valid circuit data!"
		kafka.SubmitResult(result)
		return
	}
	log.Debugf("Subscriber (%s) Site data is %v", subscriber, site)
	PON = site.CircuitData[0].PON
	// Check for PON assignment - return an error if we don't have PON data yet
	if PON == "" {
		log.Errorf("site (%s) does not have PON data", subscribe.SiteID)
		result.Result = fmt.Sprintf("PON data missing for site (%s)", subscribe.SiteID)
		kafka.SubmitResult(result)
		return
	}
	// Check to see if they have any Internet services
	pools := map[string]bool{}
	reservations := map[string]dhcpdb.Reservation{}
	var services []mcp.OLTService

	// see which products have a network profile to add them as a service
	for _, product := range request.Products {
		var productData maxbill.Product
		productData, err = maxbill.GetProduct(CoreDB, "product_code", product.ProductCode)
		if err != nil {
			log.Errorf("getting maxbill product (%s) - %v", product.ProductCode, err)
			result.Result = fmt.Sprintf("Problem getting maxbill product (%s) - %v", product.ProductCode, err)
			kafka.SubmitResult(result)
			continue
		}
		// only provision products with a network profile field; determines the network provisioning template
		if productData.NetworkProfile != nil {
			// this is where we check routing_node
			profile := *productData.NetworkProfile
			if profile.AddressPool != "" {
				pools[profile.AddressPool] = true
				log.Debugf("Added %v to the dhcp pool list", profile.AddressPool)
			}
			// begin to construct the MCP Service from the Product
			var servicedata mcp.OLTService
			servicedata.ProductData = productData
			if product.SubProductCode != "" {
				var subscribeservicearray []maxbill.SubscribedProduct
				log.Debugf("Getting services - %v", product.SubProductCode)
				subscribeservicearray, err = maxbill.GetServices(CoreDB, []telmax.Filter{telmax.Filter{Key: "subprod_code", Value: product.SubProductCode}})
				if err != nil {
					log.Errorf("getting subscribed product (%s) details %v", product.SubProductCode, err)
					result.Result = fmt.Sprintf("Problem getting subscribed product (%s) details %v", product.SubProductCode, err)
					kafka.SubmitResult(result)
					continue
				}
				if len(subscribeservicearray) < 1 {
					log.Errorf("subscribed product (%s) returned no results", product.SubProductCode)
					result.Result = fmt.Sprintf("Internet provision Error - subprod_code (%s) does not exist!", product.SubProductCode)
					kafka.SubmitResult(result)
					continue
				}
				servicedata.SubscribeProduct = subscribeservicearray[0]
				servicedata.Name = subscriber + "-" + servicedata.SubscribeProduct.SubProductCode

				services = append(services, servicedata)
			}
		}
	}
	// the basic ONT information
	var (
		hasONT    bool
		isGpon    bool
		activeONT mcp.ONTData
	)
	// Looks for any ONT in the device record, RG are handled elsewhere
	for _, device := range request.Devices {
		log.Debugf("device request data is %v", device)
		if device.DeviceType == "AccessTerminal" {
			//var definition devices.DeviceDefinition
			definition, err := devices.GetDeviceDefinition(CoreDB, "devicedefinition_code", device.DefinitionCode)
			if err != nil {
				log.Errorf("getting device definition (%s) - %v", device.DefinitionCode, err)
				result.Result = fmt.Sprintf("Problem getting device definition (%s) - %v", device.DefinitionCode, err)
				kafka.SubmitResult(result)
				continue
			}
			log.Debugf("device definition is %v", definition)
			// Adjust this if other devices or upstream interfaces need to be supported
			switch {
			case definition.Vendor == "AdTran" && definition.Upstream == "XGSPON":
				log.Debugf("Found XGSPON ONT")
				activeONT.Definition = definition
				activeONT.Device, err = devices.GetDevice(CoreDB, "device_code", device.DeviceCode)
				if err == nil {
					hasONT = true
				} else {
					log.Errorf("getting device (%s) - %v", definition.Model, err)
					result.Result = fmt.Sprintf("Problem getting device (%s) - %v", definition.Model, err)
					kafka.SubmitResult(result)
					continue
				}
			case definition.Vendor == "AdTran" && definition.Upstream == "GPON":
				log.Debugf("Found GPON ONT")
				isGpon = true
				activeONT.Definition = definition
				activeONT.Device, err = devices.GetDevice(CoreDB, "device_code", device.DeviceCode)
				if err == nil {
					hasONT = true
				} else {
					log.Errorf("getting device (%s) - %v", definition.Model, err)
					result.Result = fmt.Sprintf("Problem getting device (%s) - %v", definition.Model, err)
					kafka.SubmitResult(result)
					continue
				}
			}
		}
	}
	// return if no ONT found
	if !hasONT {
		log.Debugf("no ONT in device provision request, nothing to do")
		return
	}

	var (
		ONU int
		CP  string // ContentProvider
	)
	// Assign a circuit and ONU ID
	circuit, assigned, err := netdb.AllocateCircuit(NetDB, site.WireCentre, PON, subscriber)
	if err != nil {
		log.Errorf("assigning circuit (%s)(%s)(%s) - %v", site.WireCentre, PON, subscriber, err)
		result.Result = fmt.Sprintf("Problem assigning circuit (%s)(%s)(%s) - %v", site.WireCentre, PON, subscriber, err)
		result.Success = false
		kafka.SubmitResult(result)
		return
	}
	// assigned reflects whether work was done, ie a new assignment was completed
	if !assigned {
		log.Infof("Circuit (%s) was already assigned", circuit.ID)
		result.Result = fmt.Sprintf("Re-using existing circuit ID (%s)", circuit.ID)
		result.Success = true
		kafka.SubmitResult(result)
	}
	// Set variables for the circuit and content provider strings
	ONU = circuit.Unit
	CP = circuit.AccessNode + "-cp"
	log.Infof("ONU (%d) on Content-Provider (%s) from Circuit (%s) assigned to Subscriber (%s)", ONU, CP, circuit.ID, subscriber)
	result.Result = fmt.Sprintf("Assigned Circuit (%s) to Subscriber (%s)", circuit.ID, subscriber)
	result.Success = true
	kafka.SubmitResult(result)
	// revert result.Success back to false to let future successes toggle it
	result.Success = false

	// modify the PON interface if GPON... arbitrary naming convention
	// messes up the Circuit Allocation! must be altered after
	// and not reflected in the network.access_ports DB
	if isGpon {
		tmp := strings.Split(PON, "-")
		if len(tmp) < 2 {
			log.Errorf("unexpected PON (%s)", PON)
			result.Result = fmt.Sprintf("unexpected PON (%s)", PON)
			kafka.SubmitResult(result)
			return
		}
		// stouffville-olt01-pon01 then stouffville-olt01-gpon01
		// OR sandiford-lab-olt01-pon01 then sandiford-lab-olt01-gpon01
		// OR olt01-pon01 then olt01-gpon01
		tmp[len(tmp)-1] = "g" + tmp[len(tmp)-1]
		PON = strings.Join(tmp, "-")
	}
	// Create the ONT and interfaces in MCP
	err = mcp.CreateONT(subscriber, activeONT, PON, ONU)
	if err != nil {
		// logged error within function
		result.Result = err.Error()
		kafka.SubmitResult(result)
		return
	}
	result.Result = "Created ONT and interface objects"
	result.Success = true
	kafka.SubmitResult(result)
	result.Success = false

	// Get DHCP leases for each service that needs one  This is based on the network profile, if a DHCP pool is listed
	log.Debugf("Allocating addresses in DHCP pools %v", pools)
	for pool := range pools {
		var resulttext string
		// the pool like likely be "residential"
		reservations[pool], err = dhcpdb.DhcpAssign(circuit.RoutingNode, pool, subscriber)
		if err != nil {
			resulttext = fmt.Sprintf("Problem assigning address (%s) - %v", pool, err)
			result.Success = false
		} else {
			resulttext = fmt.Sprintf("Assigned address (%s) from pool (%s) with VLAN (%d)", reservations[pool].V4Addr.String(), pool, reservations[pool].VlanID)
			result.Success = true
		}
		result.Result = resulttext
		kafka.SubmitResult(result)
	}
	result.Success = false
	// Add services
	for _, service := range services {
		// You need the vlan ID from the reservation to know which VLAN the customer should be connected to
		// the address pool is most likely "residential"
		if service.ProductData.NetworkProfile.AddressPool != "" {
			service.Vlan = reservations[service.ProductData.NetworkProfile.AddressPool].VlanID
			log.Infof("Vlan ID is %v", service.Vlan)
		} else {
			service.Vlan = service.ProductData.NetworkProfile.Vlan
		}
		if service.ProductData.Category == "Internet" {
			log.Debugf("creating Internet service %v", service.ProductData.NetworkProfile.ProfileName)
			if service.ProductData.NetworkProfile.ProfileName == "" {
				log.Errorf("Service %v does not have a network profile!", service.Name)
			}
			// HARDCODED PORT NUMBER..?
			err = mcp.CreateDataService(service.Name, subscriber+"-ONT", subscriber, service.ProductData.NetworkProfile.ProfileName, CP, service.Vlan, 1)
			if err != nil {
				log.Errorf("creating service (%s) - %v", service.Name, err)
				result.Result = fmt.Sprintf("Problem creating service (%s) - %v", service.Name, err)
				result.Success = false
				kafka.SubmitResult(result)
			} else {
				log.Infof("created service object (%s) - %v", service.Name, service)
				result.Result = "Created service object " + service.Name
				result.Success = true
				kafka.SubmitResult(result)
			}
		} else {
			log.Infof("unexpected service type - %v", service.ProductData.Category)
		}

	}
	// iterate through the phone numbers on the ONT device record
	for _, voicesvc := range activeONT.Device.VoiceServices {
		if voicesvc.Username != "" {
			log.Infof("Creating voice service for DID (%s)", voicesvc.Username)
			var (
				profile   string
				voicevlan int
			)
			// Right now, we only have residential, but we could use the domain to determine some config options
			switch voicesvc.Domain {
			case "residential.telmax.ca":
				profile = "telmax-res-voice"
				voicevlan = 202 // HARDCODED VLAN... EEP
			default:
				profile = "telmax-res-voice"
				voicevlan = 202 // HARDCODED VLAN... EEP
			}
			var did DID
			// Get the DID information from the telephone database
			did, err := GetDID(voicesvc.Username)
			if err != nil {
				log.Errorf("Problem getting voice DID %v", err)
				result.Result = fmt.Sprintf("Could not get DID (%s) from telephone API - %v", voicesvc.Username, err)
				result.Success = false
			} else if did.UserData == nil {
				result.Result = fmt.Sprintf("Missing user data for DID (%s)", voicesvc.Username)
				result.Success = false
			} else {
				log.Infof("DID data for (%s) is %v", voicesvc.Username, did)
				// Create a voice service in MCP, now that we have all the information we need
				err = mcp.CreatePhoneService(subscriber+"-"+voicesvc.Username, subscriber+"-ONT", subscriber, profile, CP, voicevlan, did.Number, did.UserData.SIPPassword, int(voicesvc.Line))
				if err != nil {
					result.Result = "Problem adding voice service " + err.Error()
					result.Success = false
				} else {
					result.Result = fmt.Sprintf("Added DID (%s) to FXS port (%d)", voicesvc.Username, int(voicesvc.Line))
					result.Success = true
				}
			}
			kafka.SubmitResult(result)
		}
	}
}

// Unprovision services - used for cancelling a customer, or backing out provisioning (wrong PON or other re-do)
// This is similar to provision, but a bit simpler and only removes the services.  Could also apply for a cancellation
// of a subset of services, but not the whole thing.
// Does not unprovision Voice.
func UnProvisionServices(request telmaxprovision.ProvisionRequest) {
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}
	subscribe, err := maxbill.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("getting subscriber (%s)(%s) - %v", request.AccountCode, request.SubscribeCode, err)
		result.Result = fmt.Sprintf("Problem getting subscriber (%s)(%s) - %v", request.AccountCode, request.SubscribeCode, err)
		kafka.SubmitResult(result)
		return
	}
	if subscribe.NetworkType != "Fibre" {
		log.Debugf("nothing to do here")
		return
	}
	subscriber := subscribe.AccountCode + "-" + subscribe.SubscribeCode
	//		var site Site
	//var circuit netdb.Circuit
	circuit, err := netdb.GetSubscriberCircuit(CoreDB, subscriber)
	if err != nil {
		log.Errorf("getting subscriber circuit (%s) - %v", subscriber, err)
		result.Result = fmt.Sprintf("Problem getting subscriber circuit (%s) - %v", subscriber, err)
		kafka.SubmitResult(result)
		return
	}
	// Make a list of the services we need to remove  Mostly, we just need the name
	for _, product := range request.Products {
		var productData maxbill.Product
		//			var servicedata OLTService
		productData, err = maxbill.GetProduct(CoreDB, "product_code", product.ProductCode)
		if err != nil {
			log.Errorf("getting maxbill product (%s) - %v", product.ProductCode, err)
			result.Result = fmt.Sprintf("Problem getting maxbill product (%s) - %v", product.ProductCode, err)
			kafka.SubmitResult(result)
			continue
		}
		name := subscriber + "-" + product.SubProductCode
		var success bool
		if productData.NetworkProfile != nil {
			// If there was a DHCP pool, go and release the address back to the pool
			if productData.NetworkProfile.AddressPool != "" {
				success, err = dhcpdb.DhcpRelease(circuit.RoutingNode, productData.NetworkProfile.AddressPool, subscriber)
				if err != nil {
					log.Errorf("releasing subscribed circuit DHCP (%s) %v", productData.NetworkProfile.AddressPool, err)
					result.Result = fmt.Sprintf("Problem releasing subscribed circuit DHCP (%s) %v", productData.NetworkProfile.AddressPool, err)
					result.Success = false
					kafka.SubmitResult(result)
				} else if success {
					log.Infof("removed DHCP lease from pool (%s)", productData.NetworkProfile.AddressPool)
				} else {
					log.Errorf("no error but was unsuccessful removing DHCP lease from pool (%s)", productData.NetworkProfile.AddressPool)
				}
			}
		}
		// Moved this block out of the NetworkProfile section to allow unprovisioning Voice services.
		// Delete the service object
		err = mcp.DeleteService(name)
		if err != nil {
			log.Errorf("deleting service (%s) - %v", name, err)
			result.Result = fmt.Sprintf("Problem deleting service (%s) - %v", name, err)
			result.Success = false
		} else {
			result.Success = true
			result.Result = fmt.Sprintf("Removed service (%s)", name)
			if success {
				result.Result += " and released DHCP binding."
			}
		}
		kafka.SubmitResult(result)
	}
}

// Remove the ONT and interfaces
func DeleteONT(request telmaxprovision.ProvisionRequest) {
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}
	subscribe, err := maxbill.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("getting subscriber (%s)(%s) - %v", request.AccountCode, request.SubscribeCode, err)
		result.Result = fmt.Sprintf("Problem getting subscriber (%s)(%s) - %v", request.AccountCode, request.SubscribeCode, err)
		kafka.SubmitResult(result)
		return
	}
	if subscribe.NetworkType != "Fibre" {
		log.Debugf("nothing to do here")
		return
	}
	subscriber := subscribe.AccountCode + "-" + subscribe.SubscribeCode
	name := subscriber + "-ONT"
	//		var site Site
	var circuit netdb.Circuit
	// Get the circuit so we can clean it up properly
	circuit, err = netdb.GetSubscriberCircuit(CoreDB, subscriber)
	if err != nil {
		log.Errorf("getting subscriber circuit (%s) - %v", subscriber, err)
		result.Result = fmt.Sprintf("Problem getting subscriber circuit (%s) - %v", subscriber, err)
		kafka.SubmitResult(result)
		return
	}
	var ONT mcp.ONTData
	for _, device := range request.Devices {
		definition, err := devices.GetDeviceDefinition(CoreDB, "devicedefinition_code", device.DefinitionCode)
		if err != nil {
			log.Errorf("getting device definition (%s) - %v", device.DefinitionCode, err)
			result.Result = fmt.Sprintf("Problem getting device definition (%s) - %v", device.DefinitionCode, err)
			kafka.SubmitResult(result)
			continue
		}
		if definition.Vendor == "AdTran" {
			ONT.Definition = definition
		}
	}
	err = mcp.DeleteONT(subscriber, ONT)
	if err != nil {
		log.Errorf("deleting ONT (%s) %v", name, err)
		result.Result = fmt.Sprintf("Problem deleting ONT (%s) %v", name, err)
		result.Success = false
	} else {
		// Release the circuit back to the pool
		err = netdb.ReleaseCircuit(CoreDB, circuit.ID)
		if err == nil {
			result.Success = true
			result.Result = "Removed service " + name
		} else {
			log.Errorf("releasing circuit (%s) - %v", circuit.ID, err)
			result.Result = fmt.Sprintf("releasing circuit (%s) - %v", circuit.ID, err)
			result.Success = false
		}
	}
	kafka.SubmitResult(result)
}

// Handle an ONT swap through update mechanisms and reflow job
func DeviceSwap(request telmaxprovision.ProvisionRequest) {
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}
	subscribe, err := maxbill.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("getting subscriber (%s)(%s) - %v", request.AccountCode, request.SubscribeCode, err)
		result.Result = fmt.Sprintf("Problem getting subscriber (%s)(%s) - %v", request.AccountCode, request.SubscribeCode, err)
		kafka.SubmitResult(result)
		return
	}
	if subscribe.NetworkType != "Fibre" {
		log.Debugf("nothing to do here")
		return
	}
	subscriber := subscribe.AccountCode + "-" + subscribe.SubscribeCode
	//		var site Site
	//		var PON string

	// Get the ONT information

	var hasONT bool
	var activeONT mcp.ONTData
	for _, device := range request.Devices {
		log.Debugf("Device data is %v", device)
		if device.DeviceType == "AccessTerminal" {
			var definition devices.DeviceDefinition
			definition, err = devices.GetDeviceDefinition(CoreDB, "devicedefinition_code", device.DefinitionCode)
			if err != nil {
				log.Errorf("getting device definition (%s) - %v", device.DefinitionCode, err)
				result.Result = fmt.Sprintf("Problem getting device definition (%s) - %v", device.DefinitionCode, err)
				kafka.SubmitResult(result)
				continue
			}
			log.Debugf("Device definition is %v", definition)
			if definition.Vendor == "AdTran" && definition.Upstream == "XGSPON" {
				log.Infof("Found XGS-PON ONT")
				activeONT.Definition = definition
				activeONT.Device, err = devices.GetDevice(CoreDB, "device_code", device.DeviceCode)
				if err != nil {
					log.Errorf("getting device (%s) - %v", device.DeviceCode, err)
					result.Result = fmt.Sprintf("Problem getting device (%s) - %v", device.DeviceCode, err)
					kafka.SubmitResult(result)
					continue
				}
				hasONT = true
			}
			if definition.Vendor == "AdTran" && definition.Upstream == "GPON" {
				log.Infof("Found GPON ONT")
				activeONT.Definition = definition
				activeONT.Device, err = devices.GetDevice(CoreDB, "device_code", device.DeviceCode)
				if err != nil {
					log.Errorf("getting device (%s) - %v", device.DeviceCode, err)
					result.Result = fmt.Sprintf("Problem getting device (%s) - %v", device.DeviceCode, err)
					kafka.SubmitResult(result)
					continue
				}
				hasONT = true
			}
		}
	}
	// Update ONT record
	if hasONT {
		// Update the ONT record in MCP
		err = mcp.UpdateONT(subscriber, activeONT)
		if err != nil {
			log.Errorf("updating ONT (%s) - %v", subscriber, err)
			result.Result = fmt.Sprintf("Problem updating ONT (%s) - %v", subscriber, err)
			kafka.SubmitResult(result)
			return
		} else {
			log.Infof("Updated ONT (%s) object and queued re-flow job", subscriber)
			result.Result = fmt.Sprintf("Updated ONT (%s) object and queued re-flow job", subscriber)
			result.Success = true
			kafka.SubmitResult(result)
		}
	}
}
