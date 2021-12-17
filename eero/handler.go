package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"
	"bitbucket.org/telmaxnate/eero"
	//"strings"
	"strconv"
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
	log.Infof("Got provision request %v", request)

	switch request.RequestType {
	case "New":
		// add one or more device and create a new network
		//NewRequest(request)
		NewEero(request)

	case "Update":
		// add one or more device to the existing network OR
		// add one or more device and create a new network (was smart-rg)
		//NewRequest(request)
		NewEero(request)

	case "DeviceReturn":
		// Remove device but do not delete network
		// assume there are other devices using it or will use it
		log.Info("Handling returned devices")
		//DeviceReturn(request)
		EeroReturn(request)

	case "Cancel":
		// CancelSubscription
		// Remove device and delete network
		log.Info("Removing cancelled subscription including devices and associated networks")
		EeroCancel(request)
	}
}

func NewEero(request telmaxprovision.ProvisionRequest) {
	var isEero bool
	var hasNet bool
	var deviceSns []string // using device codes instead of Mac addresses
	var networkID int
	for _, device := range request.Devices {
		if device.DeviceType == "RG" {
			if eero.IsDeviceCode(device.DefinitionCode) {
				isEero = true
				if device.Serial == "" {
					dev, err := telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
					if err != nil {
						log.Errorf("Received NIL SN and cannot resolve device from DeviceCode=%s - %v", device.DeviceCode, err)
					} else {
						deviceSns = append(deviceSns, dev.Serial)
					}
				} else {
					deviceSns = append(deviceSns, device.Serial)
				}
			}
		}
	}
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}
	if isEero {
		//
		subscribe, err := telmax.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
		if err != nil {
			log.Errorf("Problem getting subscriber %v", err)
			return
		} else {
			// verbose, but debugging will let us know what the db entry looked like before
			log.Info(subscribe)
		}
		// Check if subscribe.ACSSubscriber is a valid Eero Network, and if it is do not create one.
		// This will handle the Update Provision in the same place as New Device.
		if subscribe.ACSSubscriber != 0 {
			net, err := eeroApi.GetNetworkById(subscribe.ACSSubscriber)
			if err == nil {
				hasNet = true
				log.Warnf("Subscribe has a valid network %d - not creating a new one", subscribe.ACSSubscriber)
				// check if network SSID is the same as the subscribe record?
				if subscribe.LanSSID != net.Name {
					// prefer this method since subscribe is most likely auto-generated
					// if the on-site is different, it may have been changed by preference
					// unlikely that the network will exist already but be incorrect
					// if that was the case, could update it here instead...?
					// Net object does not reveal PSK, can only "PUT" it.
					log.Warnf("Subscribe has different SSID '%s' than provisioned network '%s' - updating record to match network. PSK may be incorrect!", subscribe.LanSSID, net.Name)
					subscribe.LanSSID = net.Name
				}
			} else {
				// error will either bet 403 or 404, depending on int length
				// in either case, make a new network.
				log.Infof("Value of Subscribe.ACSSubscriber=%d is not a valid or active Eero Network - %v", subscribe.ACSSubscriber, err)
			}
		}
		if !hasNet {
			// create a new network
			if subscribe.LanSSID == "" {
				subscribe.GenerateSSID()
			}
			if subscribe.LanPassphrase == "" {
				subscribe.LanPassphrase = telmax.RandString(12)
			}
			net, err := eeroApi.CreateDefaultNetwork(subscribe.LanSSID, subscribe.LanPassphrase)
			if err != nil {
				log.Errorf("Error creating new network with SSID=%s PSK=%s - %v", subscribe.LanSSID, subscribe.LanPassphrase, err)
				result.Result = fmt.Sprintf("Problem creating new network with SSID=%s PSK=%s - %v", subscribe.LanSSID, subscribe.LanPassphrase, err)
				kafka.SubmitResult(result)
			} else {
				id, err := strconv.Atoi(eero.LastUrlSegment(net.Url))
				if err != nil {
					log.Errorf("Error updating the subscribe.ACSSubscriber ID (int) with the last segment of the Eero Network URL %s - %v", net.Url, err)
					result.Result = fmt.Sprintf("Problem updating the subscribe.ACSSubscriber ID (int) with the last segment of the Eero Network %s - %v", net.Url, err)
					kafka.SubmitResult(result)
				} else {
					subscribe.ACSSubscriber = id
					log.Infof("Created new network %d with SSID=%s PSK=%s", id, subscribe.LanSSID, subscribe.LanPassphrase)
				}
			}
			// can bind Eero Network.CustomerAccount.PartnerAccountId to some Telmax internal logic; must implement in Eero package
		}
		err = subscribe.Update(CoreDB)
		// keep in-memory copy since thre subscribe object goes away outside of this loop
		networkID = subscribe.ACSSubscriber
		if err != nil {
			log.Errorf("Problem updating subscribe %v", err)
			result.Result = "Problem creating Eero Network Updating Subscriber record" + err.Error()
			kafka.SubmitResult(result)
		}
	}
	// Device section
	for _, sn := range deviceSns {
		var (
			snid     string
			devOs    string
			devModel string
		)
		dev, err := eeroApi.PostNewEero(sn, "", "")
		if err != nil {
			// error could be that device already exists
			devSearch, err := eeroApi.GetEeroBySn(sn)
			if err == nil {
				if devSearch.Network.Url != "" {
					if eero.LastUrlSegment(devSearch.Network.Url) == strconv.Itoa(networkID) {
						log.Infof("Device SN=%s is already configured for Network ID=%d", sn, networkID)
						result.Result = fmt.Sprintf("Device SN=%s is already configured for Network ID=%d", sn, networkID)
						kafka.SubmitResult(result)
						continue
					} else {
						// device has network but not proper one
						log.Infof("Device SN=%s already exists but is configured for the incorrect network %s - overwriting", sn, devSearch.Network.Url)
						// Can't patch over, must remove Eero and readd!
						err = eeroApi.DeleteEeroBySn(sn)
						if err != nil {
							log.Infof("Device SN=%s is already configured for wrong Network and can't be deleted - %v ID=%d", sn, err)
							result.Result = fmt.Sprintf("Device SN=%s  is already configured for wrong Network and can't be deleted - %v ID=%d", sn, err)
							kafka.SubmitResult(result)
							continue
						} else {
							log.Infof("Device SN=%s deleted so it can be added properly", sn)
						}
						dev, err = eeroApi.PostNewEero(sn, "", "")
						if err != nil {
							log.Infof("Device SN=%s failed to be reassigned after deletion - %v", sn, err)
							result.Result = fmt.Sprintf("Device SN=%s failed to be reassigned after deletion - %v", sn, err)
							kafka.SubmitResult(result)
							continue
						}
						log.Infof("Device SN=%s posted to Eero API as %s", sn, dev.Url)
						snid = eero.LastUrlSegment(dev.Url)
						devOs = dev.Os
						devModel = dev.Model
					}
				} else {
					// device exists but doesn't have network
					log.Infof("Device SN=%s already exists but doesn't have a network - setting", sn)
					snid = strconv.Itoa(devSearch.Id)
					devModel = devSearch.Model
				}
			} else {
				// can't post new device and can't find existing device record
				log.Errorf("Problem posting new Eero with SN=%s - %v", sn, err)
				result.Result = fmt.Sprintf("Problem adding device with SN=%s - %v", sn, err)
				kafka.SubmitResult(result)
				continue
			}
		} else {
			log.Infof("Device SN=%s posted to Eero API as %s", sn, dev.Url)
			snid = eero.LastUrlSegment(dev.Url)
			devOs = dev.Os
			devModel = dev.Model
		}
		// decouple different object references
		device, err := telmax.GetDevice(CoreDB, "serial", sn)
		device.Firmware = devOs
		device.DeviceModel = devModel
		// location is passed as "" because why does it matter, and how would we know in pre-provision?
		_, err = eeroApi.UpdateEero(sn, fmt.Sprintf("/2.2/networks/%d", networkID), "", snid)
		if err != nil {
			// Error could be that SNID="", meaning the URL format parse failed
			log.Errorf("Problem updating Eero SN=%s SNID=%s with Network NetID=%d - %v", sn, snid, networkID, err)
			result.Result = fmt.Sprintf("Problem updating Eero SN=%s SNID=%s with Network NetID=%d - %v", sn, snid, networkID, err)
			kafka.SubmitResult(result)
		} else {
			// update the device record with new info
			err = device.Update(CoreDB)
			if err != nil {
				log.Errorf("Problem updating Device record for Eero SN=%s - %v", sn, err)
				result.Result = fmt.Sprintf("Problem updating Device record for Eero SN=%s - %v", sn, err)
				kafka.SubmitResult(result)
			} else {
				log.Infof("Successfully added device SN=%s to Eero Network %d as %s", sn, networkID, snid)
				result.Success = true
				result.Time = time.Now()
				result.Result = fmt.Sprintf("Successfully added device SN=%s to Eero Network %d as %s", sn, networkID, snid)
				kafka.SubmitResult(result)
			}
		}
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
				if device.Serial == "" {
					dev, err := telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
					if err != nil {
						log.Errorf("Received NIL SN and cannot resolve device from DeviceCode=%s - %v", device.DeviceCode, err)
						return
					} else {
						sn = dev.Serial
					}
				} else {
					sn = device.Serial
				}
				err := eeroApi.DeleteEeroBySn(sn)
				if err != nil && err.Error() != "404 Not Found" {
					log.Errorf("Problem removing Eero SN=%s - %v", sn, err)
				} else {
					log.Infof("Eero SN=%s has been removed from any associated networks", sn)
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
				if device.Serial == "" {
					dev, err := telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
					if err != nil {
						log.Errorf("Received NIL SN and cannot resolve device from DeviceCode=%s - %v", device.DeviceCode, err)
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
					log.Errorf("Problem finding Eero SN=%s in API - %v", sn, err)
				} else {
					// tie back network to Mac address, using a map to identify unique values
					network[devSearch.Network.Url] = append(network[devSearch.Network.Url], sn)
					err = eeroApi.DeleteEeroBySn(sn)
					if err != nil && err.Error() != "404 Not Found" {
						log.Errorf("Problem removing Eero SN=%s - %v", sn, err)
					} else {
						log.Infof("Eero SN=%s has been removed from any associated networks", sn)
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
			log.Errorf("Problem deleting network %s - %v", url, err)
		} else {
			log.Infof("Eero network %s deleted", eero.LastUrlSegment(url))
		}
	}
}
