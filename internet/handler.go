/*
	Internet provisioning handler for telMAX distributed provisioning system

	This service will consume provision requests and execute network provisioning to enable
	Internet services on FTTx networks using AdTran MCP.  This service also interacts with
	the circuit database to assign and maintain access circuit records, as well as the DHCP
	database to allocate IP address resources to customers.
*/
package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"

	//"strings"
	"bitbucket.org/telmaxdc/telmax-common"
	"bitbucket.org/telmaxdc/telmax-common/devices"
	"bitbucket.org/telmaxdc/telmax-common/maxbill"
	"bitbucket.org/telmaxdc/telmax-provision/dhcpdb"
	"bitbucket.org/telmaxdc/telmax-provision/kafka"
	"bitbucket.org/telmaxdc/telmax-provision/mcp"
	"bitbucket.org/telmaxdc/telmax-provision/netdb"
	"bitbucket.org/telmaxdc/telmax-provision/structs"
	"strconv"
	"time"
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

	// Generate a new result object
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}

	subscribe, err := maxbill.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("Problem getting subscriber %v", err)
		result.Result = err.Error()
		kafka.SubmitResult(result)

	}

	// We only act on this if the subscription is Fibre
	if subscribe.NetworkType == "Fibre" {
		subscriber := subscribe.AccountCode + "-" + subscribe.SubscribeCode
		var site Site
		var PON string

		// Check to see if there is a site ID  If not, we can't really proceed
		if subscribe.SiteID != "" {
			site, err = GetSite(subscribe.SiteID)
			if err != nil {
				log.Errorf("Problem getting site %v", err)
				result.Result = "Problem getting site data from QGIS " + err.Error()
				kafka.SubmitResult(result)
				return
			}
			log.Infof("Site data is %v", site)
			if len(site.CircuitData) < 1 {
				result.Result = "This site does not have valid circuit data!"
				kafka.SubmitResult(result)
				return
			}
			PON = site.CircuitData[0].PON

			//		if subscribe.Wirecentre == "" {
		} else {
			result.Result = "Subscriber site ID not set - mandatory!"
			kafka.SubmitResult(result)
			return
		}

		// Check to see if they have any Internet services
		pools := map[string]bool{}
		reservations := map[string]dhcpdb.Reservation{}
		var services []mcp.OLTService

		// We only need to provision products that have a network profile field.  This determines the network provisioning template
		log.Warn("Iterating through products to see which ones have a network profile")
		for _, product := range request.Products {
			var productData maxbill.Product
			var servicedata mcp.OLTService
			productData, err = maxbill.GetProduct(CoreDB, "product_code", product.ProductCode)
			if productData.NetworkProfile != nil {
				profile := *productData.NetworkProfile
				if profile.AddressPool != "" {
					pools[profile.AddressPool] = true
					log.Infof("Adding %v to the dhcp pool list", profile.AddressPool)
				}
				servicedata.ProductData = productData
				if product.SubProductCode != "" {
					var subscribeservicearray []maxbill.SubscribedProduct

					log.Infof("Getting services - %v", product.SubProductCode)
					subscribeservicearray, err = maxbill.GetServices(CoreDB, []telmax.Filter{telmax.Filter{Key: "subprod_code", Value: product.SubProductCode}})
					if err != nil {
						log.Errorf("Problem getting subscribed product details %v", err)
						result.Result = err.Error()
						result.Success = false
						kafka.SubmitResult(result)
					} else {
						if len(subscribeservicearray) == 1 {
							servicedata.SubscribeProduct = subscribeservicearray[0]
							servicedata.Name = subscriber + "-" + servicedata.SubscribeProduct.SubProductCode
						} else {
							log.Errorf("Could not get product for subprod_code %v", product.SubProductCode)
							result.Result = "Internet provision Error - subprod_code " + product.SubProductCode + " does not exist!"
							result.Success = false
							kafka.SubmitResult(result)
						}
					}
				} else {
					servicedata.Name = subscriber + "-" + product.Category
				}

				services = append(services, servicedata)

			}
		}

		// Get the ONT information

		var hasONT bool
		var activeONT mcp.ONTData

		// Go through the devices in the request and see if any of them are AdTran ONTs  This could be modified for other qualifying devices
		for _, device := range request.Devices {
			log.Infof("Device data is %v", device)
			if device.DeviceType == "AccessTerminal" {
				var definition devices.DeviceDefinition
				definition, err = devices.GetDeviceDefinition(CoreDB, "devicedefinition_code", device.DefinitionCode)
				log.Infof("Device definition is %v", definition)
				// Adjust this if other devices or upstream interfaces need to be supported
				if definition.Vendor == "AdTran" && definition.Upstream == "XGSPON" {
					log.Infof("Found ONT")
					activeONT.Definition = definition
					activeONT.Device, err = devices.GetDevice(CoreDB, "device_code", device.DeviceCode)
					hasONT = true
				}

			}
		}
		// Create ONT record - don't bother if we didn't find an ONT in the device list
		if hasONT {
			var ONU int
			var CP string

			// Check for PON assignment - return an error if we don't have PON data yet
			if PON == "" {
				log.Errorf("Site %v does not have PON data", site)
				result.Result = "PON data missing for site"
				result.Success = false
				kafka.SubmitResult(result)
			} else {
				// Assign a circuit and ONU ID
				circuit, assigned, err := netdb.AllocateCircuit(NetDB, site.WireCentre, PON, subscriber)
				if err != nil {
					log.Errorf("Problem assigning circuit %v", err)
					result.Result = err.Error()
					result.Success = false
					kafka.SubmitResult(result)
					return
				} else {
					if !assigned {
						log.Infof("Circuit was already assigned")
						result.Result = "Re-using existing circuit ID" + circuit.ID
						result.Success = true
						kafka.SubmitResult(result)
					}
					// Set variables for the circuit and content provider strings
					ONU = circuit.Unit
					CP = circuit.AccessNode + "-cp"
					log.Infof("ONU and CP is %v %v", ONU, CP)
					result.Result = "Assigned circuit " + circuit.ID + " ONU " + strconv.Itoa(circuit.Unit)
					result.Success = true
					kafka.SubmitResult(result)
				}
				// Create the ONT record in MCP  This will also create the ONT interfaces
				err = mcp.CreateONT(subscriber, activeONT, PON, ONU)
				if err != nil {
					result.Result = err.Error()
					result.Success = false
					kafka.SubmitResult(result)

					return
				} else {
					result.Result = "Created ONT and interface objects"
					result.Success = true
					kafka.SubmitResult(result)
				}
				//				time.Sleep(30 * time.Second)

				// Get DHCP leases for each service that needs one  This is based on the network profile, if a DHCP pool is listed
				log.Warnf("Allocating addresses in DHCP pools %v", pools)
				for pool, _ := range pools {
					reservations[pool], err = dhcpdb.DhcpAssign(circuit.RoutingNode, pool, subscriber)
					var resulttext string
					if err != nil {
						resulttext = resulttext + "Problem assigning address " + err.Error() + "\n"
						result.Success = false
					} else {
						resulttext = resulttext + "Assigned address " + reservations[pool].V4Addr.String() + " from pool " + pool + " vlan " + strconv.Itoa(reservations[pool].VlanID) + "\n"
						result.Success = true
					}
					result.Result = resulttext
					kafka.SubmitResult(result)
				}

				// Add services
				for _, service := range services {

					// You need the vlan ID from the reservation to know which VLAN the customer should be connected to
					if service.ProductData.NetworkProfile.AddressPool != "" {
						service.Vlan = reservations[service.ProductData.NetworkProfile.AddressPool].VlanID
						log.Infof("Vlan ID is %v", service.Vlan)
					} else {
						service.Vlan = service.ProductData.NetworkProfile.Vlan
					}
					switch service.ProductData.Category {
					// Configure for an Internet service
					case "Internet":
						log.Infof("creating Internet service %v", service.ProductData.NetworkProfile.ProfileName)
						if service.ProductData.NetworkProfile.ProfileName != "" {
							err = mcp.CreateDataService(service.Name, subscriber+"-ONT", subscriber, service.ProductData.NetworkProfile.ProfileName, CP, service.Vlan, 1)
						} else {
							log.Errorf("Service %v does not have a network profile!", service.Name)
						}
						break
						// Phone is done a different way.  That said, we could add a "data" service for IP phones at some point.
					case "Phone":
						break
					}
					if err != nil {
						log.Errorf("Problem creating service %v - %v", service.Name, err)
						result.Result = err.Error()
						result.Success = false
						kafka.SubmitResult(result)
					} else {
						result.Result = "Created service object " + service.Name
						result.Success = true
						kafka.SubmitResult(result)

					}
				}

				// Add POTS ports - iterate through the phone numbers on the ONT device record
				for _, voicesvc := range activeONT.Device.VoiceServices {
					if voicesvc.Username != "" {
						log.Infof("Creating voice service for DID %v", voicesvc.Username)
						var profile string
						var voicevlan int

						// Right now, we only have residential, but we could use the domain to determine some config options
						switch voicesvc.Domain {
						case "residential.telmax.ca":
							profile = "telmax-res-voice"
							voicevlan = 202
							break
						default:
							profile = "telmax-res-voice"
							voicevlan = 202
							break
						}
						var did DID
						// Get the DID information from the telephone database
						did, err = GetDID(voicesvc.Username)
						log.Infof("DID data is %v", did)
						if err != nil {
							log.Errorf("Problem getting voice DID %v", err)
							result.Success = false
							result.Result = "Could not get DID " + voicesvc.Username + " from telephone API"
						} else if did.UserData == nil {
							result.Success = false
							result.Result = "Missing user data for DID " + voicesvc.Username
						} else {
							// Create a voice service in MCP, now that we have all the information we need
							err = mcp.CreatePhoneService(subscriber+"-"+voicesvc.Username, subscriber+"-ONT", subscriber, profile, CP, voicevlan, did.Number, did.UserData.SIPPassword, int(voicesvc.Line))
							if err != nil {
								result.Result = "Problem adding voice service " + err.Error()
								result.Success = false
							} else {
								result.Result = "Added DID " + voicesvc.Username + " to FXS port " + strconv.Itoa(int(voicesvc.Line))
								result.Success = true
							}
						}
						kafka.SubmitResult(result)
					}
				}
			}
		}

	} else {
		log.Info("Not a fibre customer")
	}
	return
}

// Unprovision services - used for cancelling a customer, or backing out provisioning (wrong PON or other re-do)
// This is similar to provision, but a bit simpler and only removes the services.  Could also apply for a cancellation
// of a subset of services, but not the whole thing.
func UnProvisionServices(request telmaxprovision.ProvisionRequest) {
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}

	subscribe, err := maxbill.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("Problem getting subscriber %v", err)
		result.Result = err.Error()
		kafka.SubmitResult(result)

	}
	if subscribe.NetworkType == "Fibre" {
		subscriber := subscribe.AccountCode + "-" + subscribe.SubscribeCode
		//		var site Site
		var circuit netdb.Circuit

		circuit, err = netdb.GetSubscriberCircuit(CoreDB, subscriber)
		// Make a list of the services we need to remove  Mostly, we just need the name
		for _, product := range request.Products {
			var productData maxbill.Product
			//			var servicedata OLTService
			var success bool
			productData, err = maxbill.GetProduct(CoreDB, "product_code", product.ProductCode)
			if productData.NetworkProfile != nil {
				name := subscriber + "-" + product.SubProductCode
				// If there was a DHCP pool, go and release the address back to the pool
				if productData.NetworkProfile.AddressPool != "" {
					success, err = dhcpdb.DhcpRelease(circuit.RoutingNode, productData.NetworkProfile.AddressPool, subscriber)
					if err != nil {
						log.Errorf("Problem getting subscribed product details %v", err)
						result.Result = err.Error()
						result.Success = false
						kafka.SubmitResult(result)
					} else if success == true {
						result.Result = "Removed DHCP lease from pool " + productData.NetworkProfile.AddressPool
					}
				}
				// Delete the service object
				err = mcp.DeleteService(name)
				if err != nil {
					log.Errorf("Problem deleting service %v %v", name, err)
					result.Result = err.Error()
					result.Success = false
					kafka.SubmitResult(result)
				} else if success == true {
					result.Success = true
					result.Result = "Removed service " + name
				}
				kafka.SubmitResult(result)

			}
		}

	} else {
		log.Info("Not a fibre customer")
	}
	return
}

// Remove the ONT and interfaces
func DeleteONT(request telmaxprovision.ProvisionRequest) {
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}

	subscribe, err := maxbill.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("Problem getting subscriber %v", err)
		result.Result = err.Error()
		kafka.SubmitResult(result)

	}
	if subscribe.NetworkType == "Fibre" {
		subscriber := subscribe.AccountCode + "-" + subscribe.SubscribeCode
		name := subscriber + "-ONT"
		//		var site Site
		var circuit netdb.Circuit
		// Get the circuit so we can clean it up properly
		circuit, err = netdb.GetSubscriberCircuit(CoreDB, subscriber)
		var ONT mcp.ONTData
		// We are assuming a 622V in all cases.  There is no harm if the interfaces don't exist
		ONT.Definition.EthernetPorts = 2
		ONT.Definition.PotsPorts = 2
		err = mcp.DeleteONT(subscriber, ONT)
		if err != nil {
			log.Errorf("Problem deleting ONT %v %v", name, err)
			result.Result = err.Error()
			result.Success = false
			kafka.SubmitResult(result)
		} else {
			// Release the circuit back to the pool
			netdb.ReleaseCircuit(CoreDB, circuit.ID)
			result.Success = true
			result.Result = "Removed service " + name
		}
		kafka.SubmitResult(result)

	} else {
		log.Info("Not a fibre customer")
	}
	return
}

// Handle an ONT swap through update mechanisms and reflow job
func DeviceSwap(request telmaxprovision.ProvisionRequest) {
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}

	subscribe, err := maxbill.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("Problem getting subscriber %v", err)
		result.Result = err.Error()
		kafka.SubmitResult(result)

	}
	if subscribe.NetworkType == "Fibre" {
		subscriber := subscribe.AccountCode + "-" + subscribe.SubscribeCode
		//		var site Site
		//		var PON string

		// Get the ONT information

		var hasONT bool
		var activeONT mcp.ONTData
		for _, device := range request.Devices {
			log.Infof("Device data is %v", device)
			if device.DeviceType == "AccessTerminal" {
				var definition devices.DeviceDefinition
				definition, err = devices.GetDeviceDefinition(CoreDB, "devicedefinition_code", device.DefinitionCode)
				log.Infof("Device definition is %v", definition)
				if definition.Vendor == "AdTran" && definition.Upstream == "XGSPON" {
					log.Infof("Found ONT")
					activeONT.Definition = definition
					activeONT.Device, err = devices.GetDevice(CoreDB, "device_code", device.DeviceCode)
					hasONT = true
				}

			}
		}
		// Update ONT record
		if hasONT {

			// Update the ONT record in MCP
			err = mcp.UpdateONT(subscriber, activeONT)
			if err != nil {
				result.Result = err.Error()
				kafka.SubmitResult(result)
				return
			} else {
				result.Result = "Updated ONT object and queued re-flow job"
				result.Success = true
				kafka.SubmitResult(result)
			}

		}

	} else {
		log.Info("Not a fibre customer")
	}
	return
}
