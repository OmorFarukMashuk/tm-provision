package main

import (
	"time"

	"bitbucket.org/telmaxdc/telmax-common/lab"

	log "github.com/sirupsen/logrus"
)

// auditDaemon learns about the network then periodically checks to see if there
// are any actions that must be taken
func auditDaemon() {
	eeroSummary := eeroApi.RetrieveEeroNetworkSummary()
	//lab.SendToZabbix(lab.DefaultZabbixSetup("eero-summary", eeroSummary)
	for {
		eeroApi.UpdateMissingNetworkLabels(CoreDB, DhcpDB, eeroSummary)
		eeroApi.TransferNetworks(CoreDB, eeroSummary)
		//eeroApi.GenerateStatusReport()
		//eeroApi.
		if lab.OffHours() {
			eeroApi.UpdateFirmwareBulk(eeroSummary)
		}
		time.Sleep(time.Duration(15 * time.Minute))
		eeroSummary, err = eeroApi.UpdateNetworkSummary(eeroSummary)
		if err != nil {
			log.Errorf("updating network summary - %v", err)
		}
	}
}
