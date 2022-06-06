package main

import (
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"bitbucket.org/telmaxdc/telmax-common/devices"
	"bitbucket.org/telmaxdc/telmax-common/lab"
	"bitbucket.org/telmaxdc/telmax-common/maxbill"
	"bitbucket.org/telmaxnate/eero"

	"bitbucket.org/timstpierre/telmax-provision/kafka"
	telmaxprovision "bitbucket.org/timstpierre/telmax-provision/structs"
)

// for retries on failures
var sleepTimer = time.Duration(60)
var networkPrefix = "https://dashboard.eero.com/networks/"

func HandleProvision(request telmaxprovision.ProvisionRequest) {

	switch request.RequestType {
	case "New":
		// add one or more device and create a new network
		log.Info("Eero Handler inspecting New Device request")
		// put a guard on the number of loops... for now
		for n := 0; n < 3; n++ {
			if NewEero(request) {
				break
			}
			log.Debugf("Sleeping for %d seconds\n", sleepTimer)
			// if provision is not successful, sleep and retry
			time.Sleep(sleepTimer * time.Second)
		}
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

// ProvisionHandler is the custom object that carries the information required
// for provisioning Eeros
type ProvisionHandler struct {
	NetId       int            // Network ID
	HomeId      string         // "label", concat of AcctCode + SubscribeCode
	EeroSerials []string       // collects Eero Serial Numbers in Provision Request
	SnToSnid    map[string]int // API creates a SNID that is used in place of SN in some calls
	SnToNetid   map[string]int // binding of SN to Network ID once assigned
	Results     []string       // collect all results and publish to Kafka as one entry to reduce false positives
}

// NewEero handles 'New' and 'Update' provisioning requests
// what about returning a bool and looping the handler until it returns true?
func NewEero(request telmaxprovision.ProvisionRequest) bool {

	var eeroSerials []string
	for _, device := range request.Devices {
		if device.DeviceType == "RG" {
			if eero.IsDeviceCode(device.DefinitionCode) {
				if device.Serial == "" {
					dev, err := devices.GetDevice(CoreDB, "device_code", device.DeviceCode)
					if err != nil {
						log.Errorf("Received NIL SN and cannot resolve device from Device Code (%s) - %v", device.DeviceCode, err)
					} else {
						eeroSerials = append(eeroSerials, strings.ToUpper(dev.Serial))
					}
				} else {
					eeroSerials = append(eeroSerials, strings.ToUpper(device.Serial))
				}
			}
		}
	}
	if len(eeroSerials) == 0 {
		log.Infof("No Eeros in Provision Request")
		return true
	}

	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}
	// instantiate maps to prevent panic
	ph := &ProvisionHandler{
		EeroSerials: eeroSerials,
		HomeId:      request.AccountCode + request.SubscribeCode,
		SnToSnid:    make(map[string]int),
		SnToNetid:   make(map[string]int),
	}

	// retrieve the subscribe DB entry for this provision request
	subscribe, err := maxbill.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
	if err != nil {
		log.Errorf("getting Subscribe (%s-%s) from CoreDB - %v", request.AccountCode, request.SubscribeCode, err)
		result.Result = fmt.Sprintf("Error getting Subscribe (%s-%s) from CoreDB - %v", request.AccountCode, request.SubscribeCode, err)
		kafka.SubmitResult(result)
		// return here, there is only one subscribe account per provision request
		// failed access to the Core DB is a fatal error that prevents provisoning, will block
		return false
	}
	// Check if subscribe.ACSSubscriber is a valid Eero Network, and if it IS do not create one.
	// This will handle the Update Provision in the same place as New Device.
	if subscribe.ACSSubscriber != 0 {
		// netEeroRsp contains a list of all the Eeros on a given network
		netEeroRsp, err := eeroApi.GetNetworkEeros(subscribe.ACSSubscriber)
		if err != nil {
			log.Infof("failed to retrieve Eero Network by Subscribe.ACSSubscriber (%d) - %v", subscribe.ACSSubscriber, err)
			ph.Results = append(ph.Results, fmt.Sprintf("Existing network (%s%d) unreachable (%v), creating a new network", networkPrefix, subscribe.ACSSubscriber, err))
			// as long as ph.NetId is not set, the next if block will create a new network
		} else {
			// network exists, do not create a new one but learn more about it
			ph.NetId = subscribe.ACSSubscriber
			net, err := eeroApi.GetNetworkById(ph.NetId)
			if err != nil {
				log.Infof("GetNetworkEeros succeeded but GetNetworkById failed with - %v", err)
				// shouldn't happen, but doesn't break anything. Simple check combines with logic below to null out a problematic network.
			}
			if len(netEeroRsp.Data) == 0 {
				// network exists but no Eeros attached. Did GetNetworkById fail?
				if net == nil {
					// just make a new network!
					ph.NetId = 0
					log.Infof("GetNetworkEeros returned no Eeros and GetNetworkById failed. Creating a new network")
					ph.Results = append(ph.Results, fmt.Sprintf("Existing network (%s%d) deemed unreliable, creating a new network", networkPrefix, subscribe.ACSSubscriber))
				} else {
					// Only possible if network had eeros provisioned but they were removed and the network wasn't deleted.
					// if the API call resolves, this network should be able to be used!
					log.Infof("Retrieved valid Network (%s%d) from Subscribe record that contained zero devices. Attempting to use", networkPrefix, ph.NetId)
					ph.Results = append(ph.Results, fmt.Sprintf("Valid Network (%s%d) already exists, not creating a new one", networkPrefix, ph.NetId))
				}
			} else {
				// identify which Eeros already belong to the correct network
			network:
				for i := 0; i < len(netEeroRsp.Data); i++ {
					for n := 0; n < len(ph.EeroSerials); n++ {
						if netEeroRsp.Data[i].Serial == ph.EeroSerials[n] {
							// populate what we know about the device
							ph.SnToSnid[netEeroRsp.Data[i].Serial] = eero.LastUrlSegmentInt(netEeroRsp.Data[i].Url)
							ph.SnToNetid[netEeroRsp.Data[i].Serial] = ph.NetId
							continue network
						}
					}
				}
				switch len(ph.SnToNetid) {
				case len(ph.EeroSerials):
					// all serials have been mapped to the netid
					log.Infof("Subscribe database entry contained valid Network (%s%d) which all provision request Eeros already belong to.", networkPrefix, ph.NetId)
					result.Result = fmt.Sprintf("All Eeros already belong to valid Network (%s%d)", networkPrefix, ph.NetId)
					result.Success = true
					result.Time = time.Now()
					kafka.SubmitResult(result)
					return true
				case 0:
					// no serials have been mapped to the netid
					log.Infof("Subscribe database entry contained valid Network (%s%d) which contained none of the provision request Eeros already belong to. Provisioning now", networkPrefix, ph.NetId)
					ph.Results = append(ph.Results, fmt.Sprintf("Valid Network (%s%d) already exists, not creating a new one", networkPrefix, ph.NetId))
				default:
					// some but not all are provisioned for the correct network
					log.Infof("Subscribe database entry contained valid Network (%s%d) which [%d] provision request Eeros already belong to. Provisioning the remainder", networkPrefix, ph.NetId, len(ph.SnToNetid))
					ph.Results = append(ph.Results, fmt.Sprintf("Valid Network (%s%d) already exists with [%d/%d] provision request Eeros already assigned. Provisioning the remainder.", networkPrefix, ph.NetId, len(ph.SnToNetid), len(ph.EeroSerials)))
				}
			}
		}
	}
	if ph.NetId == 0 {
		// create a new network
		if subscribe.LanSSID == "" {
			subscribe.GenerateSSID()
		}
		// create a random PSK if one doesn't exist or isn't long enough
		if subscribe.LanPassphrase == "" || len(subscribe.LanPassphrase) < 8 {
			subscribe.LanPassphrase = lab.RandString(12)
		}
		// create network returns network ID
		net, err := eeroApi.CreateDefaultNetwork(subscribe.LanSSID, subscribe.LanPassphrase)
		if err != nil {
			log.Errorf("creating new network (SSID %s)(PSK %s) - %v", subscribe.LanSSID, subscribe.LanPassphrase, err)
			result.Result = fmt.Sprintf("Error creating new network (SSID %s)(PSK %s) - %v", subscribe.LanSSID, subscribe.LanPassphrase, err)
			result.Time = time.Now()
			kafka.SubmitResult(result)
			// if new network can't be created, and one doesn't exist
			// zero out the subscribe SSID & PSK to change the potential for next provision success
			subscribe.LanSSID = ""
			subscribe.LanPassphrase = ""
			err = subscribe.Update(CoreDB)
			if err != nil {
				log.Errorf("Error Updating Database (Subscribe) after Error creating a new network - %v", err)
			} else {
				log.Infof("Nulled SSID & PSK on Subscribe record")
			}
			return false
		}
		// utility strips the URL and converts to int
		ph.NetId = eero.LastUrlSegmentInt(net.Url)
		if ph.NetId == 0 {
			// fringe event where net object returns but contains a URL that does not resolve to int
			log.Errorf("Failed to create a valid Eero Network (NetID==0)(URL %s). Exiting...", net.Url)
			result.Result = fmt.Sprintf("Error creating new network (SSID %s)(PSK %s) - invalid URL %s", subscribe.LanSSID, subscribe.LanPassphrase, net.Url)
			result.Time = time.Now()
			kafka.SubmitResult(result)
			return false
		}
		log.Infof("Created New Network (%s%d) (SSID %s)(PSK %s)", networkPrefix, ph.NetId, subscribe.LanSSID, subscribe.LanPassphrase)
		ph.Results = append(ph.Results, fmt.Sprintf("Created New Network (%s%d) (SSID %s)(PSK %s)", networkPrefix, ph.NetId, subscribe.LanSSID, subscribe.LanPassphrase))
		// anything else to add to the subscribe?
		subscribe.ACSSubscriber = ph.NetId
		err = subscribe.Update(CoreDB)
		if err != nil {
			log.Errorf("Problem Updating Subscribe on CoreDB after Successful creation of a new network - %v", err)
			ph.Results = append(ph.Results, fmt.Sprintf("Error updating Subscribe on CoreDB - %v", err))
		}
	}
device:
	// Device section: if one or more devices is an Eero that doesn't already belong to the prescribed network
	for _, sn := range ph.EeroSerials {
		// if sn already has a netid, skip provisioning
		for k := range ph.SnToNetid {
			if k == sn {
				continue device
			}
		}
		// if Eero is configured for the wrong network, delete it
		devSearch, err := eeroApi.GetEeroBySn(sn)
		if err == nil {
			if devSearch.Network.Url == "" {
				log.Infof("Eero (SN %s) does not have a Network configured", sn)
			} else {
				// if the network is configured for the desired value already (check above failed!)
				tmpNetId := eero.LastUrlSegmentInt(devSearch.Network.Url)
				if tmpNetId == ph.NetId {
					log.Infof("Eero (SN %s) is already configured for the desired Network (%d), but was not detected by GetNetworkEeros", sn, ph.NetId)
					//ph.Results = append(ph.Results, fmt.Sprintf("Eero (SN %s) is already configured for the desired Network (ID %d), but was not detected by GetNetworkEeros", sn, ph.NetId))
					ph.SnToNetid[sn] = ph.NetId
					continue device
				} else {
					// device has network but not proper one
					log.Infof("Eero (SN %s) is configured for the wrong Network (%d) - overwriting", sn, tmpNetId)
					// Can't patch over, must remove Eero and readd!
					err = eeroApi.DeleteEeroBySn(sn)
					if err != nil {
						log.Errorf("Eero (SN %s) is configured for the Wrong Network (%d) and trying to delete returns this error - %v", sn, tmpNetId, err)
						ph.Results = append(ph.Results, fmt.Sprintf("Eero (SN %s) is configured for the Wrong Network (%d) and trying to delete returns this error - %v", sn, tmpNetId, err))
						continue device
					} else {
						log.Infof("Eero (SN %s) deleted from existing Network (%d) so it can be configured for correct network", sn, tmpNetId)
					}
				}
			}
		} else {
			// devSearch had an error. Must exit, this call works for all devices we own, used by inventory
			log.Errorf("Eero (SN %s) search failed with - %v", sn, err)
			ph.Results = append(ph.Results, fmt.Sprintf("Eero (SN %s) is not found in the Eero Insight portal - %v", sn, err))
			continue device
		}
		// each Eero must be "Posted" to the API, even though it is already known to the API (as shown with GetbySn)
		dev, err := eeroApi.PostNewEero(sn)
		if err != nil {
			// most obvious error is already handled: if it already belong to a network
			// abort this device provision
			log.Errorf("Eero (SN %s) post failed with - %v", sn, err)
			ph.Results = append(ph.Results, fmt.Sprintf("Eero (SN %s) interaction with Eero Insight portal failed - %v", sn, err))
			continue device
		}
		ph.SnToSnid[sn] = eero.LastUrlSegmentInt(dev.Url)
		log.Infof("Eero (SN %s) posted to Eero API (SNID %d)", sn, ph.SnToSnid[sn])
		// location is passed as "" because it doesn't matter and we can't know in pre-provision
		// object is disregarded as it does not reflect the addition of the network and does not provide value
		_, err = eeroApi.UpdateEero(sn, "", ph.NetId, ph.SnToSnid[sn])
		if err != nil {
			log.Errorf("assigning Eero (SN %s)(SNID %d) to Network (%d) - %v", sn, ph.SnToSnid[sn], ph.NetId, err)
			ph.Results = append(ph.Results, fmt.Sprintf("Error assigning Eero (SN %s)(SNID %d) to Network (%d) - %v", sn, ph.SnToSnid[sn], ph.NetId, err))
			continue device
		}
		ph.SnToNetid[sn] = ph.NetId
		log.Infof("added Eero (SN %s) (SNID %d) to Network (%d)", sn, ph.SnToSnid[sn], ph.NetId)
		ph.Results = append(ph.Results, fmt.Sprintf("Assigned Eero (SN %s)(SNID %d) to Network (%d)", sn, ph.SnToSnid[sn], ph.NetId))
		// get the device to update it with provision outcome
		device, err := devices.GetDevice(CoreDB, "serial", sn)
		if err != nil {
			log.Errorf("getting Device record by Serial (SN %s) from CoreDB - %v", sn, err)
			ph.Results = append(ph.Results, fmt.Sprintf("Error getting Device record for Eero (SN %s) from CoreDB - %v", sn, err))
			// only reason this would fail is due to connectivity issue with DB...?
			continue device
		}
		// update the device record with everything we know
		device.Firmware = dev.Os
		device.DeviceModel = dev.Model
		device.Accountcode = request.AccountCode
		device.Subscribecode = request.SubscribeCode
		device.Location = "Account"
		device.RefreshAddress = subscribe.Address.Street // bson:address
		device.RefreshDate = time.Now()
		//device.Status = "Activate" --> managed by Billing!

		// update the device record with new info
		err = device.Update(CoreDB)
		if err != nil {
			log.Errorf("Problem updating Device record in CoreDB for Eero (SN %s) - %v", sn, err)
			ph.Results = append(ph.Results, fmt.Sprintf("Error updating Device record in CoreDB for Eero (SN %s) - %v", sn, err))
		}
	}
	// HomeID binding now done by Audit after network comes up
	/*
		// this assertion proves or disproves whether the network exists
		err = eeroApi.PutNetworkLabel(ph.NetId, ph.HomeId)
		if err != nil {
			log.Errorf("Error assigning Home Identifier (%s) to Network (%d) - %v", ph.HomeId, ph.NetId, err)
			// this assertion we
			ph.Results = append(ph.Results, fmt.Sprintf("Assigning Home Identifier (%s) to Network (%d) failed with - %v", ph.HomeId, ph.NetId, err))
		} else {
			label, err := eeroApi.GetNetworkLabel(ph.NetId)
			if err != nil || label != ph.HomeId {
				log.Errorf("Unsuccessful assigning Home Identifier (%s) to Network (%d)", ph.HomeId, ph.NetId)
				ph.Results = append(ph.Results, fmt.Sprintf("Unsuccessful assigning Home Identifier (%s) to Network (%d)", ph.HomeId, ph.NetId))
			} else {
				log.Infof("Network (%d) updated with Home Identifier (%s)", ph.NetId, ph.HomeId)
				ph.Results = append(ph.Results, fmt.Sprintf("Network (%d) updated with Home Identifier (%s)", ph.NetId, ph.HomeId))

			}
		}
	*/
	var fail bool
	// check if all provision eeros received networks
assert:
	for _, sn := range ph.EeroSerials {
		for k := range ph.SnToNetid {
			if k == sn {
				continue assert
			}
		}
		fail = true
	}
	result.Success = !fail
	result.Result = "Eero provisioning summary:\n"

	for _, res := range ph.Results {
		result.Result += fmt.Sprintf("\t-%s\n", res)
	}
	result.Time = time.Now()
	kafka.SubmitResult(result)
	return result.Success
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
					dev, err := devices.GetDevice(CoreDB, "device_code", device.DeviceCode)
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
					dev, err := devices.GetDevice(CoreDB, "device_code", device.DeviceCode)
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
		err := eeroApi.DeleteNetwork(eero.LastUrlSegmentInt(url))
		if err != nil {
			log.Errorf("Problem deleting network (ID %d) - %v", eero.LastUrlSegmentInt(url), err)
		} else {
			log.Infof("Eero network (ID %d) deleted", eero.LastUrlSegmentInt(url))
		}
	}
}
