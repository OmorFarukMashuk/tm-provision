package main

import (
	"time"

	"bitbucket.org/telmaxdc/telmax-common/lab"
)

// auditDaemon learns about the network then periodically checks to see if there
// are any actions that must be taken
// refreshing its database every hour
// and performing daily tasks during off-peak hours
func auditDaemon() {
	//start := time.Now()
	eeroApi.UpdateEeroDatabase() // run on init
	for {                        // forever
		lastDay := time.Now()
		for { // day timer
			lastHour := time.Now()
			for { // hour timer
				if time.Since(lastHour) > time.Duration(60*time.Minute) {
					eeroApi.UpdateEeroDatabase()
					eeroApi.UpdateMissingNetworkLabels(CoreDB, DhcpDB)
					eeroApi.TransferNetworks(CoreDB)
					break
				}
			}
			// more than 24 hours since last time but only in off-peak hours window
			if time.Since(lastDay) > time.Duration(24*time.Hour) && lab.OffHours() {
				eeroApi.UntransferPendingNetworks()
				eeroApi.FirmwareUpdateNetworks()
				eeroApi.LatestSpeedTests()
				eeroApi.RemoveDerelictNetworks()
				break
			}
		}
		// TODO
		// send results to zabbix
		// expose metrics
		// graceful shutdown
	}
}
