package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"

	//"strings"
	"bitbucket.org/telmaxdc/telmax-common"
	"bitbucket.org/telmaxdc/telmax-common/devices"
	"bitbucket.org/telmaxdc/telmax-common/maxbill"
	"bitbucket.org/timstpierre/telmax-provision/dhcpdb"
	"bitbucket.org/timstpierre/telmax-provision/kafka"
	"bitbucket.org/timstpierre/telmax-provision/mcp"
	"bitbucket.org/timstpierre/telmax-provision/netdb"
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
		var site Site
		var PON string

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
		// Create ONT record
		if hasONT {
			var ONU int
			var CP string

			// Check for PON assignment
			if PON == "" {
				log.Errorf("Site %v does not have PON data", site)
				result.Result = "PON data missing for site"
				kafka.SubmitResult(result)
			} else {
				// Assign a circuit and ONU ID
				circuit, assigned, err := netdb.AllocateCircuit(NetDB, site.WireCentre, PON, subscriber)
				if err != nil {
					log.Errorf("Problem assigning circuit %v", err)
					result.Result = err.Error()
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
				// Create the ONT record in MCP
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

				// Get DHCP leases for each service that needs one
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
					if service.ProductData.NetworkProfile.AddressPool != "" {
						service.Vlan = reservations[service.ProductData.NetworkProfile.AddressPool].VlanID
						log.Infof("Vlan ID is %v", service.Vlan)
					} else {
						service.Vlan = service.ProductData.NetworkProfile.Vlan
					}
					switch service.ProductData.Category {

					case "Internet":
						log.Infof("creating Internet service %v", service.ProductData.NetworkProfile.ProfileName)
						if service.ProductData.NetworkProfile.ProfileName != "" {
							err = mcp.CreateDataService(service.Name, subscriber+"-ONT", subscriber, service.ProductData.NetworkProfile.ProfileName, CP, service.Vlan, 1)
						} else {
							log.Errorf("Service %v does not have a network profile!", service.Name)
						}
						break
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

				// Add POTS ports
				for _, voicesvc := range activeONT.Device.VoiceServices {
					if voicesvc.Username != "" {
						log.Infof("Creating voice service for DID %v", voicesvc.Username)
						var profile string
						var voicevlan int
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
						did, err = GetDID(voicesvc.Username)
						log.Infof("DID data is %v", did)
						if err != nil {
							log.Errorf("Problem getting voice DID %v", err)
							result.Result = "Could not get DID " + voicesvc.Username + " from telephone API"
						} else if did.UserData == nil {
							result.Result = "Missing user data for DID " + voicesvc.Username
						} else {
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

// Unprovision services
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

		for _, product := range request.Products {
			var productData maxbill.Product
			//			var servicedata OLTService
			var success bool
			productData, err = maxbill.GetProduct(CoreDB, "product_code", product.ProductCode)
			if productData.NetworkProfile != nil {
				name := subscriber + "-" + product.SubProductCode
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

		circuit, err = netdb.GetSubscriberCircuit(CoreDB, subscriber)
		var ONT mcp.ONTData
		ONT.Definition.EthernetPorts = 2
		ONT.Definition.PotsPorts = 2
		err = mcp.DeleteONT(subscriber, ONT)
		if err != nil {
			log.Errorf("Problem deleting ONT %v %v", name, err)
			result.Result = err.Error()
			result.Success = false
			kafka.SubmitResult(result)
		} else {
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
			//			var ONU int
			//			var CP string

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
