package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"
	"bitbucket.org/timstpierre/smartrg"
	//"strings"
	"bitbucket.org/timstpierre/telmax-common"
	"strconv"
	"telmax-provision/structs"
	//"time"
)

func HandleProvision(request telmaxprovision.ProvisionRequest) {
	log.Infof("Got provision request %v", request)

	switch request.RequestType {
	case "New":
		NewRequest(request)

	case "Update":

	case "DeviceReturn":
		log.Info("Handling returned devices")
		DeviceReturn(request)

	case "Cancel":
	}
}

func DeviceReturn(request telmaxprovision.ProvisionRequest) {
	for _, device := range request.Devices {
		if device.DeviceType == "RG" {
			record, err := smartrg.GetDeviceRecord(device.Mac)
			if err != nil {
				log.Errorf("Problem getting smartRG record for device %s, %v", device.Mac, err)
			} else {
				if len(record) == 1 {
					devicecode, _ := strconv.ParseInt(record[0].Fields.DeviceID, 10, 32)
					log.Infof("Deleting device %v from ACS", devicecode)
					err = smartrg.RemoveDevice(int(devicecode))
					if err != nil {
						log.Errorf("Problem removing device %s - %v", record[0].Fields.DeviceID, err)
					}

				}
			}

		}
	}

}

func NewRequest(request telmaxprovision.ProvisionRequest) {
	var hasRG bool
	var devices []string
	for _, device := range request.Devices {
		if device.DeviceType == "RG" {
			hasRG = true
			devices = append(devices, device.Mac)
		}
	}
	subscriberaccount := request.AccountCode + request.SubscribeCode
	if hasRG {
		subscribe, err := telmax.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
		if err != nil {
			log.Errorf("Problem getting subscriber %v", err)
			return
		}
		log.Warnf("Subscriber ACS account is %v", subscribe.ACSSubscriber)
		if subscribe.ACSSubscriber == 0 {
			log.Warn("Subscribe does not have ACS account")
			name := subscribe.FirstName + " " + subscribe.LastName
			var labels []string
			subscribe.ACSSubscriber, err = smartrg.NewSubscriber(name, subscribe.Email, subscriberaccount, labels)
			if err != nil {
				log.Errorf("Problem creating subscriber %v", err)
				return
			} else {
				err = subscribe.Update(CoreDB)
				if err != nil {
					log.Errorf("Problem updating subscribe %v", err)
				}
			}
		} else {
			log.Warn("Subscribe already has account - not creating")
		}
	}

	for _, deviceMAC := range devices {
		var devicecode int
		devicecode, err := smartrg.NewDevice(deviceMAC, subscriberaccount, "")
		if err != nil {
			log.Errorf("Problem creating device entry for mac %v, %v", deviceMAC, err)
		} else {
			log.Infof("Successfully added device %v to ACS - new code is %v", deviceMAC, devicecode)
		}

	}

}
