package main

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"
	"bitbucket.org/telmaxnate/eero"
	//"strings"

	"time"

	"bitbucket.org/timstpierre/telmax-common"
	"bitbucket.org/timstpierre/telmax-provision/kafka"
	telmaxprovision "bitbucket.org/timstpierre/telmax-provision/structs"
)

var eeroApi = &eero.EeroApi{
	Gateway: "api-user.e2ro.com",
	Key:     "15974148|12d467oiahrvacdvfv1f4jl2gs", // bound to eero@telmax.com, use CLI to generate new key if needed.
}

func HandleProvision(request telmaxprovision.ProvisionRequest) {

	switch request.RequestType {
	case "New":
		// add one or more device and create a new network
		log.Info("Eero Handler inspecting New Device request")
		NewEero(request)

	case "Update":
		// add one or more device to the existing network OR
		// add one or more device and create a new network (was smart-rg)
		log.Info("Eero Handler inspecting Update request")
		NewEero(request)

	case "DeviceReturn":
		// Remove device but do not delete network
		// assume there are other devices using it or will use it
		log.Info("Eero Handler inspecting returned devices")
		EeroReturn(request)

	case "Cancel":
		// CancelSubscription
		// Remove device and delete network
		log.Info("Eero Handler inspecting cancel subscription request")
		EeroCancel(request)
	}
}

// NewEero handles 'New' and 'Update' provisioning requests
func NewEero(request telmaxprovision.ProvisionRequest) {
	var (
		isEero        bool
		hasNet        bool
		deviceSns     []string // using device codes instead of Mac addresses
		deviceSnsCopy []string
		homeID        string // home identifier or network "label", concat of AcctCode + SubscribeCode
		subscribe     telmax.Subscribe
		device        telmax.Device
		net           *eero.Network
		dev           *eero.Eero
		err           error
	)

	for _, device := range request.Devices {
		if device.DeviceType == "RG" {
			if eero.IsDeviceCode(device.DefinitionCode) {
				isEero = true
				if device.Serial == "" {
					dev, err := telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
					if err != nil {
						log.Errorf("Received NIL SN and cannot resolve device from Device Code (%s) - %v", device.DeviceCode, err)
					} else {
						deviceSns = append(deviceSns, dev.Serial)
					}
				} else {
					deviceSns = append(deviceSns, device.Serial)
				}
			}
		}
	}
	if !isEero {
		log.Infof("No Eeros in Provision Request")
		return
	}
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}
	// copy the deviceSns, as the list may change
	deviceSnsCopy = deviceSns
	// Network section: if one or more devices is an Eero
	// if eero is already provisoned and on correct network, pop from SN list
	// homeID is the binding for telMAX view of the subscriber, devices, networks
	homeID = request.AccountCode + request.SubscribeCode
	// retrieve the subscribe DB entry for this provision request
	subscribe, err = telmax.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("Problem (%v) getting subscribe entry by Account Code (%s) Subscribe Code (%s) from CoreDB", err, request.AccountCode, request.SubscribeCode)
		result.Result = fmt.Sprintf("Problem (%v) getting subscribe entry by Account Code (%s) Subscribe Code (%s) from CoreDB", err, request.AccountCode, request.SubscribeCode)
		kafka.SubmitResult(result)
		// return here, there may be more than one device but we assert it is the same subscribe account
		// failed access to the Core DB is a fatal error that prevents provisoning
		return
	}
	// Check if subscribe.ACSSubscriber is a valid Eero Network, and if it IS do not create one.
	// This will handle the Update Provision in the same place as New Device.
	if subscribe.ACSSubscriber != 0 {
		// shortcut: since we need to know the network exists, yes, but ultimately
		// we need to know if the network has attached eeros. If the network has never
		// had an eero attached, the 'GetNetworkById()' call will fail anyway.
		// Trying to reduce the re-provisioning efforts, if the network already has any of
		// the serial numbers in the provision request, pop them from the control list
		// [!!] Aggravating that this data model does not show the SSID.
		netEeroRsp, err := eeroApi.GetNetworkEeros(subscribe.ACSSubscriber)
		if err != nil {
			if strings.HasPrefix(fmt.Sprintf("%s", err), "403") {
				// network exists but is not owned or reachable
				log.Errorf("Existing value of Subscribe.ACSSubscriber (%d) is not reachable, creating a new network.", subscribe.ACSSubscriber)
				result.Result = fmt.Sprintf("Existing value of Subscribe.ACSSubscriber (%d) is not reachable, creating a new network.", subscribe.ACSSubscriber)
				kafka.SubmitResult(result)
				result.Result = ""
			} else if strings.HasPrefix(fmt.Sprintf("%s", err), "404") {
				// not a valid eero network
				log.Errorf("Existing value of Subscribe.ACSSubscriber (%d) is not valid, creating a new network", subscribe.ACSSubscriber)
				result.Result = fmt.Sprintf("Existing value of Subscribe.ACSSubscriber (%d) is not valid, creating a new network", subscribe.ACSSubscriber)
				kafka.SubmitResult(result)
				result.Result = ""
			} else {
				// unexpected error, report but ignore
				log.Errorf("Unexpected error (%v) attempting to retrieve existing value of Subscribe.ACSSubscriber (%d), creating a new network", err, subscribe.ACSSubscriber)
				result.Result = fmt.Sprintf("Unexpected error (%v) attempting to retrieve existing value of Subscribe.ACSSubscriber (%d), creating a new network", err, subscribe.ACSSubscriber)
				kafka.SubmitResult(result)
				result.Result = ""
			}
		} else if len(netEeroRsp.Data) >= 1 {
			hasNet = true // prevents creating a new network
			// identify which Eeros already belong to the correct network
			tmpLenDeviceSns := len(deviceSns)
			for i := 0; i < len(netEeroRsp.Data); i++ {
				for n := 0; n < len(deviceSns); n++ {
					if netEeroRsp.Data[i].Serial == deviceSns[n] {
						// prevent panic if pop-off is last entry
						if len(deviceSns) == 1 {
							deviceSns = []string{}
						} else {
							// pop
							// copy last entry to current entry
							deviceSns[n] = deviceSns[len(deviceSns)-1]
							// shorten list by removing duplicated last entry
							deviceSns = deviceSns[:len(deviceSns)-1]
						}
					}
				}
			}
			log.Infof("Retrieved valid Network (ID %d) from Subscribe record. Removed [%d] Eeros from provision request as they already were provisioned", subscribe.ACSSubscriber, tmpLenDeviceSns-len(deviceSns))
			result.Result = fmt.Sprintf("Retrieved valid Network (ID %d) from Subscribe record. Removed [%d] Eeros from provision request as they already were provisioned", subscribe.ACSSubscriber, tmpLenDeviceSns-len(deviceSns))
			result.Success = true
			result.Time = time.Now()
			kafka.SubmitResult(result)
			result.Success = false
			result.Result = ""
			// All network validation and DB sync done at the end
		} else {
			// network exists but no Eeros attached
			log.Infof("Retrieved valid Network (ID %d) from Subscribe record", subscribe.ACSSubscriber)
			result.Result = fmt.Sprintf("Retrieved valid Network (ID %d) from Subscribe record", subscribe.ACSSubscriber)
			result.Success = true
			result.Time = time.Now()
			kafka.SubmitResult(result)
			result.Success = false
			result.Result = ""
		}
	}
	if !hasNet {
		// create a new network
		if subscribe.LanSSID == "" {
			subscribe.GenerateSSID()
		}
		if subscribe.LanPassphrase == "" || len(subscribe.LanPassphrase) < 8 {
			subscribe.LanPassphrase = telmax.RandString(12)
		}
		net, err = eeroApi.CreateDefaultNetwork(subscribe.LanSSID, subscribe.LanPassphrase)
		if err != nil {
			log.Errorf("Error (%v) creating new network (SSID %s) (PSK %s). Exiting provision request...", err, subscribe.LanSSID, subscribe.LanPassphrase)
			result.Result = fmt.Sprintf("Error (%v) creating new network (SSID %s) (PSK %s). Exiting provision request...", err, subscribe.LanSSID, subscribe.LanPassphrase)
			kafka.SubmitResult(result)
			result.Result = ""
			// if new network can't be created, and one doesn't exist
			// must exit or change some value as we can't provision Eeros without this
			// [??] what could cause this condition that could be handled?
			return
		}
		// store the new network in the subscribe.ACSSubscriber value (int)
		subscribe.ACSSubscriber = eero.LastUrlSegmentInt(net.Url)
		if subscribe.ACSSubscriber == 0 {
			// fringe event where net object returns but contains a URL that does not resolve to int
			log.Errorf("Failed to create a valid Eero Network (NetID == 0) (URL %s). Exiting...", net.Url)
			result.Result = fmt.Sprintf("Failed to create a valid Eero Network (NetID == 0) (URL %s). Exiting...", net.Url)
			kafka.SubmitResult(result)
			result.Result = ""
			return
		}
		log.Infof("Created New Network (ID %d) (SSID %s) (PSK %s)", subscribe.ACSSubscriber, subscribe.LanSSID, subscribe.LanPassphrase)
		result.Result = fmt.Sprintf("Created New Network (ID %d) (SSID %s) (PSK %s)", subscribe.ACSSubscriber, subscribe.LanSSID, subscribe.LanPassphrase)
		result.Success = true
		kafka.SubmitResult(result)
		result.Success = false
		result.Result = ""
	}

	// Device section: if one or more devices is an Eero that doesn't already belong to the prescribed network
	// remember that ValidateEero returns 409 if already provisioned!
	for _, sn := range deviceSns {
		var snid int
		// first, check if eero already has a network, to prevent issues
		devSearch, err := eeroApi.GetEeroBySn(sn)
		if err == nil {
			// if the device is already assigned to a network
			if devSearch.Network.Url != "" {
				// if the network is configured for the desired value already (check above failed!)
				tmpNetId := eero.LastUrlSegmentInt(devSearch.Network.Url)
				if tmpNetId == subscribe.ACSSubscriber {
					log.Infof("Eero (SN %s) is already configured for Network (ID %d)", sn, subscribe.ACSSubscriber)
					result.Result = fmt.Sprintf("Eero (SN %s) is already configured for Network (ID %d)", sn, subscribe.ACSSubscriber)
					result.Success = true
					result.Time = time.Now()
					kafka.SubmitResult(result)
					result.Success = false
					result.Result = ""
					continue
				} else {
					// device has network but not proper one
					log.Infof("Eero (SN %s) is configured for the wrong network (ID %d) - overwriting", sn, tmpNetId)
					// Can't patch over, must remove Eero and readd!
					err = eeroApi.DeleteEeroBySn(sn)
					if err != nil {
						log.Errorf("Eero (SN %s) is configured for the Wrong Network (ID %d) and trying to delete returns this error - %v", sn, tmpNetId, err)
						result.Result = fmt.Sprintf("Eero (SN %s) is configured for the Wrong Network (ID %d) and trying to delete returns this error - %v", sn, tmpNetId, err)
						kafka.SubmitResult(result)
						result.Result = ""
						continue
					} else {
						log.Infof("Eero (SN %s) deleted so it can be configured for correct network", sn)
					}
				}
			} // else, we're going to give it one, proceed
		} else {
			// devSearch had an error. Must exit, this call works for all devices we own, used by inventory
			log.Errorf("Eero (SN %s) cannot be found! - %v", sn, err)
			result.Result = fmt.Sprintf("Eero (SN %s) cannot be found! - %v", sn, err)
			kafka.SubmitResult(result)
			result.Result = ""
			continue
		}

		// each Eero must be "Posted" to the API, even though it is already known to the API (as shown with GetbySn)
		dev, err = eeroApi.PostNewEero(sn, "", "")
		if err != nil {
			// most obvious error is already handled: if it already belong to a network
			// abort this device provision
			log.Errorf("Eero (SN %s) cannot be posted! - %v", sn, err)
			result.Result = fmt.Sprintf("Eero (SN %s) cannot be posted! - %v", sn, err)
			kafka.SubmitResult(result)
			result.Result = ""
			continue
		}
		snid = eero.LastUrlSegmentInt(dev.Url)
		log.Infof("Eero (SN %s) posted to Eero API (SNID %d)", sn, snid)

		// location is passed as "" because it doesn't matter and we can't know in pre-provision
		// object is disregarded as it is a third representation of the Eero that does not add any value
		_, err = eeroApi.UpdateEero(sn, fmt.Sprintf("/2.2/networks/%d", subscribe.ACSSubscriber), "", snid)
		if err != nil {
			log.Errorf("Problem assigning Eero (SN %s) (SNID %d) to Network (NetID %d) - %v", sn, snid, subscribe.ACSSubscriber, err)
			result.Result = fmt.Sprintf("Problem assigning Eero (SN %s) (SNID %d) to Network (NetID %d) - %v", sn, snid, subscribe.ACSSubscriber, err)
			kafka.SubmitResult(result)
			continue
		}
		result.Result = fmt.Sprintf("Eero (SN %s) (SNID %d) assigned to Network (NetID %d)", sn, snid, subscribe.ACSSubscriber)
		result.Success = true
		result.Time = time.Now()
		kafka.SubmitResult(result)
		result.Success = false
		result.Result = ""

		// get the device to update it with provision outcome
		device, err = telmax.GetDevice(CoreDB, "serial", sn)
		if err != nil {
			log.Errorf("Error getting Eero (SN %s) from Database - %v", sn, err)
			result.Result = fmt.Sprintf("Problem getting Eero (SN %s) from Database - %v", sn, err)
			kafka.SubmitResult(result)
			result.Result = ""
			// if device does not exist in DB but is known to the Eero API
			// skip? or take other action
			// -> DB cleanup tool to query API and ensure DB reflects correct status
			continue
		}
		device.Firmware = dev.Os
		device.DeviceModel = dev.Model
		device.Accountcode = request.AccountCode
		device.Subscribecode = request.SubscribeCode
		device.Location = subscribe.Address.Street // bson:address
		device.Status = "Activate"

		log.Infof("Successfully added Eero (SN %s) (SNID %s) to Network (ID %d)", sn, snid, subscribe.ACSSubscriber)
		result.Result = fmt.Sprintf("Successfully added Eero (SN %s) (SNID %s) to Network (ID %d)", sn, snid, subscribe.ACSSubscriber)
		result.Success = true
		result.Time = time.Now()
		kafka.SubmitResult(result)
		result.Success = false
		result.Result = ""

		// update the device record with new info
		err = device.Update(CoreDB)
		if err != nil {
			log.Errorf("Problem updating Device record for Eero (SN %s) - %v", sn, err)
			result.Result = fmt.Sprintf("Problem updating Device record for Eero (SN %s) - %v", sn, err)
			kafka.SubmitResult(result)
			result.Result = ""
		}
	}

	// keep it simple, check the network, check the Eeros.
	net, err = eeroApi.GetNetworkById(subscribe.ACSSubscriber)
	if err != nil {
		log.Errorf("Error getting newly created network (ID %d) after provision! - %v", subscribe.ACSSubscriber, err)
	} else {
		if subscribe.LanSSID != net.Name {
			// prefer this method since subscribe's SSID is most likely auto-generated
			// if the on-site is different, it may have been changed by preference
			// unlikely that the network will exist already but be incorrect
			// Net object does not reveal PSK, can only "PUT" it.
			subscribe.LanSSID = net.Name
			log.Warnf("Databse updated to reflect on-site SSID (%s) instead of pre-provisioned value (%s)", net.Name, subscribe.LanSSID)
			result.Result = fmt.Sprintf("Databse updated to reflect on-site SSID (%s) instead of pre-provisioned value (%s)", net.Name, subscribe.LanSSID)
			result.Success = true
			result.Time = time.Now()
			kafka.SubmitResult(result)
			result.Result = ""
			result.Success = false
		}
		err = eeroApi.PutNetworkLabel(subscribe.ACSSubscriber, homeID)
		if err != nil {
			log.Errorf("Error assigning Home Identifier (%s) to Network (%d) - %v", homeID, subscribe.ACSSubscriber, err)
		} else {
			label, err := eeroApi.GetNetworkLabel(subscribe.ACSSubscriber)
			if err != nil || label != homeID {
				log.Errorf("Unsuccessful assigning Home Identifier (%s) to Network (%d)", homeID, subscribe.ACSSubscriber)
			} else {
				log.Infof("Network (%d) updated with Home Identifier (%s)", subscribe.ACSSubscriber, homeID)
				result.Result = fmt.Sprintf("Network (%d) updated with Home Identifier (%s)", subscribe.ACSSubscriber, homeID)
				result.Success = true
				result.Time = time.Now()
				kafka.SubmitResult(result)
				result.Result = ""
				result.Success = false
			}
		}
	}
	netEeroRsp, err := eeroApi.GetNetworkEeros(subscribe.ACSSubscriber)
	if err != nil {
		log.Errorf("Error getting newly Network Eero List (ID %d) after provision! - %v", subscribe.ACSSubscriber, err)
	}
	for i := 0; i < len(netEeroRsp.Data); i++ {
		for n := 0; n < len(deviceSnsCopy); n++ {
			if netEeroRsp.Data[i].Serial == deviceSnsCopy[n] {
				// prevent panic if pop-off is last entry
				if len(deviceSnsCopy) == 1 {
					deviceSns = []string{}
				} else {
					// pop
					// copy last entry to current entry
					deviceSnsCopy[n] = deviceSnsCopy[len(deviceSnsCopy)-1]
					// shorten list by removing duplicated last entry
					deviceSnsCopy = deviceSnsCopy[:len(deviceSnsCopy)-1]
				}
			}
		}
	}
	if len(deviceSnsCopy) != 0 {
		log.Errorf("New network (ID %d) does not contain one or more requested Eeros! Re-asserting", subscribe.ACSSubscriber)
		for _, sn := range deviceSnsCopy {
			err = eeroApi.DeleteEeroBySn(sn)
			if err == nil {
				dev, err = eeroApi.PostNewEero(sn, "", "")
				if err == nil {
					snid, err := eeroApi.ResolveSnToId(sn)
					if err == nil {
						_, err = eeroApi.UpdateEero(sn, fmt.Sprintf("/2.2/networks/%d", subscribe.ACSSubscriber), "", snid)
						if err == nil {
							devSearch, err := eeroApi.GetEeroBySn(sn)
							if err == nil {
								if eero.LastUrlSegmentInt(devSearch.Network.Url) == subscribe.ACSSubscriber {
									log.Infof("New network (ID %d) now contains Eero (SN %s)")
								}
							}
						}
					}
				}
			}
			log.Errorf("An error occurred attempting to assert Eero (SN %s) to Network (ID %d) - %v", sn, subscribe.ACSSubscriber, err)
		}
	}

	// wait until the end to update!
	err = subscribe.Update(CoreDB)
	if err != nil {
		log.Errorf("Problem Updating Database (Subscribe) after New/Update provision request - %v", err)
		result.Result = fmt.Sprintf("Problem Updating Database (Subscribe) after New/Update provision request - %v", err)
		kafka.SubmitResult(result)
		result.Result = ""
	}

}

// EeroReturn deletes each Eero device by Serial if it exists in the system.
// This does not remove the network the Eero was using. If devices are still
// connected to that network and not part of the Return Request, they should
// not lose access.
func EeroReturn(request telmaxprovision.ProvisionRequest) {
	// no result object returned?
	for _, device := range request.Devices {
		if device.DeviceType == "RG" {
			if eero.IsDeviceCode(device.DefinitionCode) {
				var sn string
				// DB Query until Request struct implements the Serial Number directly
				// [??] Shouldn't this return also update the Device location and status?
				if device.Serial == "" {
					dev, err := telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
					if err != nil {
						log.Errorf("Received NIL SN and cannot retrieve Database entry by Device Code (%s) - %v", device.DeviceCode, err)
						// no result in Return
						//result.Result = fmt.Sprintf("Received NIL SN and cannot retreieve Database entry by Device Code (%s) - %v", device.DeviceCode, err)
						//kafka.SubmitResult(result)
						return
					} else {
						sn = dev.Serial
					}
				} else {
					sn = device.Serial
				}
				err := eeroApi.DeleteEeroBySn(sn)
				if err != nil && err.Error() != "404 Not Found" {
					log.Errorf("Problem removing Eero (SN %s) - %v", sn, err)
				} else {
					log.Infof("Eero (SN %s) has been removed from any associated networks", sn)
				}
			}
		}
	}
}

// EeroCancel deletes each Eero device by Serial if it exists in the system.
// The network associated with the first Eero is also removed, assuming if
// there is more than one device they share the same network. If this is not the
// case, the device will still be removed but a network may be left over.
// This fails safe in case this network has other live devices attached and was
// given in error.
func EeroCancel(request telmaxprovision.ProvisionRequest) {
	// no result object returned?
	network := make(map[string][]string)
	for _, device := range request.Devices {
		if device.DeviceType == "RG" {
			if eero.IsDeviceCode(device.DefinitionCode) {
				var sn string
				// DB Query until Request struct implements the Serial Number directly
				// [??] Shouldn't this cancel also update the Device location and status?
				if device.Serial == "" {
					dev, err := telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
					if err != nil {
						log.Errorf("Received NIL SN and cannot retrieve Database entry by Device Code (%s) - %v", device.DeviceCode, err)
						return
					} else {
						sn = dev.Serial
					}
				} else {
					sn = device.Serial
				}
				// need to get the eero object to know the associated network to delete
				devSearch, err := eeroApi.GetEeroBySn(sn)
				if err != nil {
					log.Errorf("Problem finding Eero (SN %s) in Insight - %v", sn, err)
				} else {
					// tie back network to Mac address, using a map to identify unique values
					network[devSearch.Network.Url] = append(network[devSearch.Network.Url], sn)
					err = eeroApi.DeleteEeroBySn(sn)
					if err != nil && err.Error() != "404 Not Found" {
						log.Errorf("Problem removing Eero (SN %s) - %v", sn, err)
					} else {
						log.Infof("Eero (SN %s) has been removed from any associated networks", sn)
					}
				}
			}
		}
	}
	// keys in map will be unique networks
	// prevents deleting the same network twice and deciding if that error is important
	for url := range network {
		err := eeroApi.DeleteNetwork(url)
		if err != nil {
			log.Errorf("Problem deleting network (ID %d) - %v", eero.LastUrlSegmentInt(url), err)
		} else {
			log.Infof("Eero network (ID %d) deleted", eero.LastUrlSegmentInt(url))
		}
	}
}
