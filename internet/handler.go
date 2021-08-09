package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"

	//"strings"
	"bitbucket.org/timstpierre/telmax-common"
	"bitbucket.org/timstpierre/telmax-provision/dhcpdb"
	"bitbucket.org/timstpierre/telmax-provision/kafka"
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
		var services []OLTService
		log.Warn("Iterating through products to see which ones have a network profile")
		for _, product := range request.Products {
			var productData telmax.Product
			var servicedata OLTService
			productData, err = telmax.GetProduct(CoreDB, "product_code", product.ProductCode)
			if productData.NetworkProfile != nil {
				profile := *productData.NetworkProfile
				if profile.AddressPool != "" {
					pools[profile.AddressPool] = true
					log.Infof("Adding %v to the dhcp pool list", profile.AddressPool)
				}
				servicedata.ProductData = productData
				if product.SubProductCode != "" {
					var subscribeservicearray []telmax.SubscribedProduct

					log.Infof("Getting services - %v", product.SubProductCode)
					subscribeservicearray, err = telmax.GetServices(CoreDB, []telmax.Filter{telmax.Filter{Key: "subprod_code", Value: product.SubProductCode}})
					if err != nil {
						log.Errorf("Problem getting subscribed product details %v", err)
						result.Result = err.Error()
						kafka.SubmitResult(result)
					} else {
						servicedata.SubscribeProduct = subscribeservicearray[0]
						servicedata.Name = subscriber + "-" + servicedata.SubscribeProduct.SubProductCode

					}
				} else {
					servicedata.Name = subscriber + "-" + product.Category
				}

				services = append(services, servicedata)

			}
		}

		// If yes, then assign an IP address in DHCP
		log.Warn("Allocating addresses in DHCP pools")
		for pool, _ := range pools {
			reservations[pool], err = dhcpdb.DhcpAssign(site.WireCentre, pool, subscriber)
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

		var hasONT bool
		var activeONT ONTData
		for _, device := range request.Devices {
			log.Infof("Device data is %v", device)
			if device.DeviceType == "AccessTerminal" {
				var definition telmax.DeviceDefinition
				definition, err = telmax.GetDeviceDefinition(CoreDB, "devicedefinition_code", device.DefinitionCode)
				log.Infof("Device definition is %v", definition)
				if definition.Vendor == "AdTran" && definition.Upstream == "XGSPON" {
					log.Infof("Found ONT")
					activeONT.Definition = definition
					activeONT.Device, err = telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
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
				err = CreateONT(subscriber, activeONT, PON, ONU)
				if err != nil {
					result.Result = err.Error()
					kafka.SubmitResult(result)
					return
				} else {
					result.Result = "Created ONT object"
					result.Success = true
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
						err = CreateDataService(service.Name, subscriber+"-ONT", subscriber, service.ProductData.NetworkProfile.ProfileName, CP, service.Vlan, 1)
						break
					case "Phone":
						break
					}
					if err != nil {
						log.Errorf("Problem creating service %v - %v", service.Name, err)
						result.Result = err.Error()
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
							err = CreatePhoneService(subscriber+"-"+voicesvc.Username, subscriber+"-ONT", subscriber, profile, CP, voicevlan, did.Number, did.UserData.SIPPassword, int(voicesvc.Line))
							if err != nil {
								result.Result = "Problem adding voice service " + err.Error()
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

// Handle an ONT swap through update mechanisms and reflow job
func DeviceSwap(request telmaxprovision.ProvisionRequest) {
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}

	subscribe, err := telmax.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
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
		var activeONT ONTData
		for _, device := range request.Devices {
			log.Infof("Device data is %v", device)
			if device.DeviceType == "AccessTerminal" {
				var definition telmax.DeviceDefinition
				definition, err = telmax.GetDeviceDefinition(CoreDB, "devicedefinition_code", device.DefinitionCode)
				log.Infof("Device definition is %v", definition)
				if definition.Vendor == "AdTran" && definition.Upstream == "XGSPON" {
					log.Infof("Found ONT")
					activeONT.Definition = definition
					activeONT.Device, err = telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
					hasONT = true
				}

			}
		}
		// Update ONT record
		if hasONT {
			//			var ONU int
			//			var CP string

			// Update the ONT record in MCP
			err = UpdateONT(subscriber, activeONT)
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
