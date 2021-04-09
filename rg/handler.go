package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"
	"bitbucket.org/timstpierre/smartrg"
	//"strings"
	"bitbucket.org/timstpierre/telmax-common"
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
		NewRequest(request)

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
	var subscriberID int
	for _, device := range request.Devices {
		if device.DeviceType == "RG" {
			hasRG = true
			devices = append(devices, device.Mac)
		}
	}
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}
	subscriberaccount := request.AccountCode + request.SubscribeCode
	if hasRG {
		subscribe, err := telmax.GetSubscribe(CoreDB, request.AccountCode, request.SubscribeCode)
		if err != nil {
			log.Errorf("Problem getting subscriber %v", err)
			return
		}
		log.Warnf("Subscriber ACS account is %v", subscribe.ACSSubscriber)
		name := subscribe.FirstName + " " + subscribe.LastName
		if subscribe.ACSSubscriber == 0 {
			log.Warn("Subscribe does not have ACS account")
			subscribe.ACSSubscriber, err = smartrg.NewSubscriber(name, subscribe.Email, subscriberaccount)
			if err != nil {
				log.Errorf("Problem creating subscriber %v", err)
				return
			}
		} else {
			log.Warn("Subscribe already has account - not creating")
		}
		err = subscribe.Update(CoreDB)
		subscriberID = subscribe.ACSSubscriber

		if err != nil {
			log.Errorf("Problem updating subscribe %v", err)
			result.Result = "Problem creating ACS Subscriber record" + err.Error()
			kafka.SubmitResult(result)
		} else {
			var acsacct smartrg.ACSSubscriber
			acsacct, err = smartrg.GetSubscriber(subscribe.ACSSubscriber)
			if err != nil {
				log.Errorf("Problem getting subscriber for update %v", err)
				result.Result = "Problem getting ACS Subscriber record" + err.Error()
				kafka.SubmitResult(result)

			} else {
				log.Debugf("ACS Subscriber details are %v", acsacct)
				acsacct.Attributes.Email = subscribe.Email
				acsacct.Attributes.Name = name
				//acsacct.Credentials.Login = subscribe.Email
				//acsacct.Credentials.Password = "telMAXinc@5*"
				//acsacct.Credentials.Locked = false
				acsacct.Labels = []smartrg.ACSLabel{
					smartrg.ACSLabel{
						Name:     subscribe.NetworkType,
						FGColour: "#000",
						BGColour: "#fff",
					},
				}
				err = smartrg.PutSubscriber(acsacct)
				if err != nil {
					log.Errorf("Problem updating subscriber details")
					result.Result = "Problem updating ACS Subscriber record" + err.Error()
					kafka.SubmitResult(result)

				} else {
					result.Success = true
					result.Time = time.Now()
					result.Result = "Updated ACS subscriber record " + strconv.Itoa(subscriberID)
					kafka.SubmitResult(result)

				}
			}
		}
	}

	for _, deviceMAC := range devices {
		var devicecode int
		devicecode, err := smartrg.NewDevice(deviceMAC, subscriberaccount, "")
		if err != nil {
			if err.Error() == "Problem adding device OUI/SN is used by a different device." {
				log.Info("Device record exists - removing duplicate")
				record, err := smartrg.GetDeviceRecord(deviceMAC)
				log.Warnf("Duplicated device record is %v", record)
				if err != nil {
					log.Errorf("Problem getting smartRG record for device to delete duplicate %s, %v", deviceMAC, err)
					result.Result = "Problem getting smartRG record for device to delete duplicate " + deviceMAC + " Error " + err.Error()
					kafka.SubmitResult(result)
				} else {
					if len(record) == 1 {
						devicecode, _ := strconv.ParseInt(record[0].Fields.DeviceID, 10, 32)
						deviceSubscriberID := record[0].Fields.SubscriberID
						if deviceSubscriberID == "0" {
							log.Infof("Deleting device %v from ACS", devicecode)
							err = smartrg.RemoveDevice(int(devicecode))
							if err != nil {
								log.Errorf("Problem removing device %s - %v", record[0].Fields.DeviceID, err)
							} else {
								devicecode, err := smartrg.NewDevice(deviceMAC, subscriberaccount, "")
								if err != nil {
									log.Infof("Successfully added device %v to ACS - new code is %v", deviceMAC, devicecode)
									result.Success = true
									result.Time = time.Now()
									result.Result = "Added device " + deviceMAC + " to ACS"
									kafka.SubmitResult(result)

								} else {
									log.Errorf("Problem creating device entry for mac %v, %v", deviceMAC, err)
									result.Result = "Problem creating device entry for mac " + deviceMAC + " " + err.Error()
									result.Time = time.Now()
									kafka.SubmitResult(result)
								}
							}
						} else if deviceSubscriberID == strconv.Itoa(subscriberID) {
							log.Infof("Device with MAC %v is already provisioned", deviceMAC)
							result.Result = "Device " + deviceMAC + " already provisioned, skipping"
							result.Success = true
							result.Time = time.Now()
							kafka.SubmitResult(result)

						} else {
							log.Errorf("Device with MAC %v is already assigned to subscriber %v", deviceMAC, deviceSubscriberID)
							result.Result = "Device with MAC " + deviceMAC + " is already assigned to subscriber " + deviceSubscriberID
							result.Time = time.Now()
							kafka.SubmitResult(result)
						}

					}
				}

			} else {
				log.Errorf("Problem creating device entry for mac %v, %v", deviceMAC, err)
				result.Result = "Problem creating device entry for mac " + deviceMAC + " " + err.Error()
				result.Time = time.Now()
				kafka.SubmitResult(result)
			}
		} else {
			log.Infof("Successfully added device %v to ACS - new code is %v", deviceMAC, devicecode)
			result.Success = true
			result.Time = time.Now()
			result.Result = "Added device " + deviceMAC + " to ACS"
			kafka.SubmitResult(result)
		}

	}

}
