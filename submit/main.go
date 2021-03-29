package main

/*
	Configured for testing - send a structure and see what happens

*/

import (
	"bitbucket.org/timstpierre/telmax-provision/kafka"
	"bitbucket.org/timstpierre/telmax-provision/structs"
	log "github.com/sirupsen/logrus"
)

var ()

func init() {

}

func main() {
	iterations := 3
	request := telmaxprovision.ProvisionRequest{
		RequestID:     "test001",
		AccountCode:   "ACCT0001",
		AccountName:   "Test Account",
		SubscribeCode: "SUBS001",
		SiteID:        "",
		SubscribeName: "Test Subscribe",
		RequestType:   "New",
		RequestTicket: "Ticket0001",
		RequestUser:   "tstpierre",
		Products: []telmaxprovision.ProvisionProduct{
			telmaxprovision.ProvisionProduct{
				SubProductCode: "subprod0001",
				ProductCode:    "PROD0004",
				Category:       "TV",
			},
		},
		Devices: []telmaxprovision.ProvisionDevice{
			telmaxprovision.ProvisionDevice{
				DeviceCode:     "DEVI0001",
				DefinitionCode: "DEVIDEF0003",
				DeviceType:     "TV-STB",
				Mac:            "000B03030303",
			},
		},
	}

	for iterations > 0 {
		id, err := kafka.SubmitRequest(request)
		if err != nil {
			log.Error("Problem submitting request %v", err)
		}
		log.Info("Request id is %v", id)
		iterations--
	}

	kafka.Shutdown()
}
