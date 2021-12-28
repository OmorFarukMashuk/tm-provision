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

	switch request.RequestType {
	case "New":
		// add one or more device and create a new network
		log.Info("Handing New Device request")
		NewEero(request)

	case "Update":
		// add one or more device to the existing network OR
		// add one or more device and create a new network (was smart-rg)
		log.Info("Handling Update request")
		NewEero(request)

	case "DeviceReturn":
		// Remove device but do not delete network
		// assume there are other devices using it or will use it
		log.Info("Handling returned devices")
		EeroReturn(request)

	case "Cancel":
		// CancelSubscription
		// Remove device and delete network
		log.Info("Removing cancelled subscription including devices and associated networks")
		EeroCancel(request)
	}
}

// NewEero handles 'New' and 'Update' provisioning request by the
func NewEero(request telmaxprovision.ProvisionRequest) {
	var (
		isEero    bool
		hasNet    bool
		deviceSns []string // using device codes instead of Mac addresses
		homeID    string   // home identifier or network "label", concat of AcctCode + SubscribeCode
		subscribe telmax.Subscribe
		device    telmax.Device
		net       *eero.Network
		dev       *eero.Eero
		err       error
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
	// TO DO
	// understand how these fields are supposed to be populated
	// refresh them each submit so they are not carried over!
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}
	// Network section: if one or more devices is an Eero
	if isEero {
		// homeID is the binding for telMAX view of the subscriber, devices, networks
		homeID = request.AccountCode + request.SubscribeCode
		// retrieve the subscribe DB entry for this provision request
		subscribe, err = telmax.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
		if err != nil {
			log.Errorf("Problem getting subscribe entry by Account Code (%s) Subscribe Code (%s) from CoreDB - %v", request.AccountCode, request.SubscribeCode, err)
			result.Result = fmt.Sprintf("Problem getting subscribe entry by Account Code (%s) Subscribe Code (%s) from CoreDB - %v", request.AccountCode, request.SubscribeCode, err)
			kafka.SubmitResult(result)
			// return here, there may be more than one device but we assert it is the same subscribe account
			return
		} else {
			// verbose, but debugging will let us know what the db entry looked like before
			log.Info(subscribe)
			// kafka does not need to hear about successful database calls
		}
		// Check if subscribe.ACSSubscriber is a valid Eero Network, and if it IS do not create one.
		// This will handle the Update Provision in the same place as New Device.
		if subscribe.ACSSubscriber != 0 {
			net, err = eeroApi.GetNetworkById(subscribe.ACSSubscriber)
			if err == nil {
				hasNet = true
				log.Infof("Subscribe has a valid Eero network (%d) - not creating a new one", subscribe.ACSSubscriber)
				// check if network SSID is the same as the subscribe record?
				if subscribe.LanSSID != net.Name {
					// prefer this method since subscribe's SSID is most likely auto-generated
					// if the on-site is different, it may have been changed by preference
					// unlikely that the network will exist already but be incorrect
					// Net object does not reveal PSK, can only "PUT" it.
					log.Warnf("Subscribe.ACSSubscriber (%d) is valid Eero network with different SSID (%s) than provision request (%s) - updating record to match existing network. PSK may be incorrect!", subscribe.ACSSubscriber, subscribe.LanSSID, net.Name)
					result.Result = fmt.Sprintf("Subscribe.ACSSubscriber (%d) is valid Eero network with different SSID (%s) than provision request (%s) - updating record to match existing network. PSK may be incorrect!", subscribe.ACSSubscriber, subscribe.LanSSID, net.Name)
					result.Success = true
					result.Time = time.Now()
					kafka.SubmitResult(result)
					result.Result = ""
					result.Success = false
					subscribe.LanSSID = net.Name
				}
				// the label (home identifier) is how telMAX binds its account tracking to the Eero API
				// checked at the End once devices are attached, blindly SET without checking
				err = eeroApi.PutNetworkLabel(subscribe.ACSSubscriber, homeID)
				if err != nil {
					log.Warnf("Error updating Network (%d) Label. May be incorrect! - %v", subscribe.ACSSubscriber, err)
				}
			} else {
				// error will either bet 403 or 404, depending on int length
				// in either case, make a new network.
				// Exception: unmarshal error is not a response code, and means the call was successful!
				log.Errorf("Existing value of Subscribe.ACSSubscriber (%d) is not a valid or active Eero Network - %v", subscribe.ACSSubscriber, err)
				result.Result = fmt.Sprintf("Existing value of Subscribe.ACSSubscriber (%d) is not a valid or active Eero Network. Creating New Network.", subscribe.ACSSubscriber)
				kafka.SubmitResult(result)
				result.Result = ""
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
			net, err = eeroApi.CreateDefaultNetwork(subscribe.LanSSID, subscribe.LanPassphrase)
			if err != nil {
				log.Errorf("Error creating new network (SSID %s) (PSK %s) - %v", subscribe.LanSSID, subscribe.LanPassphrase, err)
				result.Result = fmt.Sprintf("Problem creating new network (SSID %s) (PSK %s) - %v", subscribe.LanSSID, subscribe.LanPassphrase, err)
				kafka.SubmitResult(result)
				result.Result = ""
			} else {
				// store the new network in the subscribe.ACSSubscriber value (int)
				id, err := strconv.Atoi(eero.LastUrlSegment(net.Url))
				if err != nil || id == 0 {
					log.Errorf("Error updating the Subscribe.ACSSubscriberID with the last segment of the Eero Network URL %s - %v", net.Url, err)
					result.Result = fmt.Sprintf("Problem updating the Database (Subscribe.ACSSubscriberID) with the new Eero Network ID %s - %v", net.Url, err)
					kafka.SubmitResult(result)
					result.Result = ""
				} else {
					subscribe.ACSSubscriber = id
					log.Infof("Created New Network (ID %d) (SSID %s) (PSK %s)", id, subscribe.LanSSID, subscribe.LanPassphrase)
					result.Result = fmt.Sprintf("Created New Network (ID %d) (SSID %s) (PSK %s)", id, subscribe.LanSSID, subscribe.LanPassphrase)
					result.Success = true
					kafka.SubmitResult(result)
					result.Success = false
					result.Result = ""
				}
			}
		}
		err = subscribe.Update(CoreDB)
		if err != nil {
			log.Errorf("Problem updating subscribe - %v", err)
			result.Result = fmt.Sprintf("Problem Updating Database (Subscribe) after New/Update provision request - %v", err)
			kafka.SubmitResult(result)
			result.Result = ""
		}
	}

	// Device section: if one or more devices is an Eero
	for _, sn := range deviceSns {
		var (
			// two separate eero objects are used if an error occurs
			// these values are the essential common elements of both
			snid     string
			devOs    string // where Os is Operating System or Firmware Version
			devModel string
		)
		// First, each Eero must be "Posted" to the API, even though it is already known to the API (will respond to GetbySn)
		dev, err = eeroApi.PostNewEero(sn, "", "")
		if err != nil {
			// error could be that device is already configured for a network, posting cannot override this, must delete then repost
			// devSearch is a separate object than "dev", the one returned from searching is different than the one returned by posting
			devSearch, err := eeroApi.GetEeroBySn(sn)
			if err == nil {
				// if the device is already assigned to a network
				if devSearch.Network.Url != "" {
					// if the network is configured for the desired value already
					if eero.LastUrlSegment(devSearch.Network.Url) == strconv.Itoa(subscribe.ACSSubscriber) {
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
						log.Infof("Eero (SN %s) is configured for the wrong network (%s) - overwriting", sn, devSearch.Network.Url)
						// Can't patch over, must remove Eero and readd!
						err = eeroApi.DeleteEeroBySn(sn)
						if err != nil {
							log.Errorf("Eero (SN %s) is already configured for wrong Network and trying to delete returns this error - %v", sn, err)
							result.Result = fmt.Sprintf("Eero (SN %s) is already configured for wrong Network and trying to delete returns this error - %v", sn, err)
							kafka.SubmitResult(result)
							result.Result = ""
							continue
						} else {
							log.Infof("Eero (SN %s) deleted so it can be configured for correct network", sn)
						}
						dev, err = eeroApi.PostNewEero(sn, "", "")
						if err != nil {
							log.Errorf("Eero (SN %s) failed trying to reassign to proper network - %v", sn, err)
							result.Result = fmt.Sprintf("Eero (SN %s) failed trying to reassign to proper network - %v", sn, err)
							kafka.SubmitResult(result)
							result.Result = ""
							continue
						}
						snid = eero.LastUrlSegment(dev.Url)
						log.Infof("Eero (SN %s) posted to Eero API (SNID %s)", sn, snid)
						// kafka positive? don't have one on first Post attempt, wait until Put Network
						devOs = dev.Os
						devModel = dev.Model
					}
				} else {
					// device exists but isn't assigned to a network
					log.Infof("Eero (SN %s) already exists but doesn't have a network - setting", sn)
					//
					snid = strconv.Itoa(devSearch.Id)
					devModel = devSearch.Model
				}
			} else {
				// can't post new device and can't find existing device record
				log.Errorf("Problem posting new Eero (SN %s) - %v", sn, err)
				result.Result = fmt.Sprintf("Problem posting new Eero (SN %s) - %v", sn, err)
				kafka.SubmitResult(result)
				result.Result = ""
				continue
			}
		} else {
			snid = eero.LastUrlSegment(dev.Url)
			log.Infof("Eero (SN %s) posted to Eero API (SNID %s)", sn, snid)
			devOs = dev.Os
			devModel = dev.Model
		}
		// location is passed as "" because it doesn't matter and we can't know in pre-provision
		// object is disregarded as it is a third representation of the Eero that does not add any value
		_, err = eeroApi.UpdateEero(sn, fmt.Sprintf("/2.2/networks/%d", subscribe.ACSSubscriber), "", snid)
		if err != nil {
			// Error could be that SNID="", meaning the URL format parse failed
			log.Errorf("Problem assigning Eero (SN %s) (SNID %s) to Network (NetID %d) - %v", sn, snid, subscribe.ACSSubscriber, err)
			result.Result = fmt.Sprintf("Problem assigning Eero (SN %s) (SNID %s) to Network (NetID %d) - %v", sn, snid, subscribe.ACSSubscriber, err)
			kafka.SubmitResult(result)
			continue
		} else {
			result.Result = fmt.Sprintf("Eero (SN %s) (SNID %s) assigned to Network (NetID %d)", sn, snid, subscribe.ACSSubscriber)
			result.Success = true
			result.Time = time.Now()
			kafka.SubmitResult(result)
			result.Success = false
			result.Result = ""
		}

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
		device.Firmware = devOs
		device.DeviceModel = devModel
		device.Accountcode = request.AccountCode
		device.Subscribecode = request.SubscribeCode
		device.Location = subscribe.Address.Street // bson:address
		device.Status = "Activate"

		// update the device record with new info
		err = device.Update(CoreDB)
		if err != nil {
			log.Errorf("Problem updating Device record for Eero SN=%s - %v", sn, err)
			result.Result = fmt.Sprintf("Problem updating Device record for Eero (SN %s) - %v", sn, err)
			kafka.SubmitResult(result)
			result.Result = ""
		} else {
			log.Infof("Successfully added Eero (SN %s) (SNID %s) to Network (ID %d)", sn, snid, subscribe.ACSSubscriber)
			result.Result = fmt.Sprintf("Successfully added Eero (SN %s) (SNID %s) to Network (ID %d)", sn, snid, subscribe.ACSSubscriber)
			result.Success = true
			result.Time = time.Now()
			kafka.SubmitResult(result)
			result.Success = false
			result.Result = ""
		}
	}

	// if one or more device has been added to a network, assert and collect

	if len(deviceSns) >= 1 {
		// assert expected values

		if subscribe.ACSSubscriber < 5000 { // arbitrary #, haven't seen a NetId lower than this
			log.Errorf("Not successful in tracking Eero Network in Database as Subscribe.ACSSubscribe (%d)")
		}
		// check each device against subscribe record ACS
		for i := 0; i < len(deviceSns); i++ {
			testDev, err := eeroApi.GetEeroBySn(deviceSns[i])
			if err != nil {
				log.Errorf("Provision device (SN %s) is not reachable after provision event!!", deviceSns[i])
				// what can we do about this? Should have errored earlier, unlikely but necessary check to avoid panic
				continue
			}
			if subscribe.ACSSubscriber != eero.LastUrlSegmentInt(testDev.Network.Url) {
				log.Errorf("Subscribe record shows Network Id (%d) but test Eero (SN %s) shows Network Id (%d)", subscribe.ACSSubscriber, deviceSns[i], eero.LastUrlSegmentInt(testDev.Network.Url))
				// fix it? wait?
			}

		}

		// add internal identifiers
		err = eeroApi.PutNetworkLabel(subscribe.ACSSubscriber, homeID)
		if err != nil {
			log.Errorf("Error assigning Home Identifier (%s) to Network (%d) - %v", homeID, subscribe.ACSSubscriber, err)
		} else {
			label, err := eeroApi.GetNetworkLabel(subscribe.ACSSubscriber)
			if err != nil || label != homeID {
				log.Errorf("Unsuccessful assigning Home Identifier (%s) to Network (%d)", homeID, subscribe.ACSSubscriber)
			} else {
				log.Infof("Network (%d) updated with Home Identifier (%s)", subscribe.ACSSubscriber, homeID)
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
				// [??] Shouldn't this return also update the Device location and status?
				if device.Serial == "" {
					dev, err := telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
					if err != nil {
						log.Errorf("Received NIL SN and cannot retreieve Database entry by Device Code (%s) - %v", device.DeviceCode, err)
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
				// [??] Shouldn't this cancel also update the Device location and status?
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
