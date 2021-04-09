package main

import (
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"
	"bitbucket.org/timstpierre/telmax-common"
	//"strings"
	"bitbucket.org/timstpierre/telmax-provision/kafka"
	"bitbucket.org/timstpierre/telmax-provision/structs"
	"telmax-provision/tv/enghouse"
	"time"
)

// Determine what sort of request it was
func HandleProvision(request telmaxprovision.ProvisionRequest) {
	log.Infof("Got provision request %v", request)

	switch request.RequestType {
	case "New":
		log.Info("Handling new TV provision request")
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
	type accountSubscribe struct {
		AccountCode   string
		SubscribeCode string
	}
	accountlist := map[string]accountSubscribe{}
	for _, device := range request.Devices {
		if device.DeviceType == "TVSetTopBox" {
			deviceData, err := telmax.GetDevice(CoreDB, "device_code", device.DeviceCode)
			if err != nil {
				log.Errorf("Problem getting device details for code %v - %v", device.DeviceCode, err)
			} else {
				if deviceData.Accountcode != "" && deviceData.Subscribecode != "" {
					log.Infof("Pushing account %v subscribe %v to accounts to refresh", deviceData.Accountcode, deviceData.Subscribecode)
					account := deviceData.Accountcode + deviceData.Subscribecode
					accountlist[account] = accountSubscribe{
						AccountCode:   deviceData.Accountcode,
						SubscribeCode: deviceData.Subscribecode,
					}
				} else {
					log.Warnf("Account information for device %v is already gone!", device.DeviceCode)
				}
			}
		}
	}

	for _, tvaccount := range accountlist {
		// Run an update on the TV account here - this will trigger a refresh without the STBs
		log.Infof("Updating TV account %v", tvaccount)
		accountdata, err := enghouse.EnghouseAccount(CoreDB, tvaccount.AccountCode, tvaccount.SubscribeCode)

		if len(accountdata.Service) > 0 {
			err = enghouse.EnghouseRequest(accountdata, request.RequestID)
			if err != nil {
				log.Errorf("Problem provisioning TV account %v", err)
			}
		} else {
			log.Infof("No Enghouse channels for account %v subscribe %v", request.AccountCode, request.SubscribeCode)
		}
	}

}

func NewRequest(request telmaxprovision.ProvisionRequest) {
	accountdata, err := enghouse.EnghouseAccount(CoreDB, request.AccountCode, request.SubscribeCode)
	if len(accountdata.Service) > 0 {
		err = enghouse.EnghouseRequest(accountdata, request.RequestID)
		result := telmaxprovision.ProvisionResult{
			RequestID: request.RequestID,
			Time:      time.Now(),
		}
		if err != nil {
			log.Errorf("Problem provisioning TV account %v", err)
			result.Result = "Problem provisioning TV Services" + err.Error()
			kafka.SubmitResult(result)
			ResultException(result, "New TV Account", false, err)

		} else {
			result.Success = true
			result.Result = "Enghouse provisioning accepted"
			kafka.SubmitResult(result)
		}
	} else {
		log.Infof("No Enghouse channels for account %v subscribe %v", request.AccountCode, request.SubscribeCode)
	}
}

func CancelRequest(request telmaxprovision.ProvisionRequest) {
	accountdata, err := enghouse.EnghouseAccount(CoreDB, request.AccountCode, request.SubscribeCode)
	//	if len(accountdata.Service) > 0 {
	accountdata.Account_status = "REMOVED"
	err = enghouse.EnghouseRequest(accountdata, request.RequestID)
	result := telmaxprovision.ProvisionResult{
		RequestID: request.RequestID,
		Time:      time.Now(),
	}
	if err != nil {
		log.Errorf("Problem cancelling TV account %v", err)
		result.Result = "Problem cancelling TV Services" + err.Error()
	} else {
		result.Success = true
		result.Result = "Enghouse cancellation accepted"
	}
	kafka.SubmitResult(result)
	//	} else {
	//		log.Infof("No Enghouse channels for account %v subscribe %v", request.AccountCode, request.SubscribeCode)
	//	}
}

func ResultException(result telmaxprovision.ProvisionResult, tag string, alert bool, err error) {
	exception := telmaxprovision.ProvisionException{
		RequestID:     result.RequestID,
		Reference:     result.Reference,
		ReferenceType: result.ReferenceType,
		Time:          time.Now(),
		System:        "tv",
		Tag:           tag,
		Alert:         alert,
		Error:         tag + " - " + err.Error(),
	}
	submiterr := kafka.SubmitException(exception)
	if submiterr != nil {
		log.Errorf("Problem submitting exception %v", submiterr)
	}
}
