package main

import (
	//	"bitbucket.org/telmaxdc/telmax-common/devices"
	"bitbucket.org/telmaxdc/telmax-common/maxbill"
	//	telmaxprovision "bitbucket.org/timstpierre/telmax-provision/structs"
	"bitbucket.org/timstpierre/telmax-provision/mcp"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

func HandleDummyTest(w http.ResponseWriter, r *http.Request) {
	CORSHeaders(w, r)
	if !CheckAuth(w, r) {
		return
	}
	requestvars := r.URL.Query()

	accountcode := mux.Vars(r)["accountcode"]
	subscribecode := mux.Vars(r)["subscribecode"]
	var user string

	if val, ok := requestvars["user"]; ok {
		user = val[0]
	} else {
		user = "unknown"
	}
	var response Response
	var subscribe maxbill.Subscribe
	var err error

	if subscribecode != "" && accountcode != "" {
		subscribe, err = maxbill.GetSubscribe(CoreDB, accountcode, subscribecode)
	} else {
		err = errors.New("You must supply an accountcode and a subscribe code!")
	}
	if err == nil {
		log.Infof("Running dummy test on %v - %v", accountcode, subscribecode)
		response.Data = TestResults{
			TestStart:   time.Now(),
			RequestUser: user,
			TestName:    "Dummy test for UI development - " + subscribe.NetworkType,
			Summary:     "Check outside plant network for poor signal",
			Results: []TestResult{
				TestResult{
					Name:         "Good Test",
					ResultString: "100% working",
					ResultData:   100,
					Pass:         true,
				},
				TestResult{
					Name:         "Bad Test",
					ResultString: "5 out of 10 packets received",
					ResultData: map[string]int{
						"sent":     10,
						"received": 5,
					},
					Pass: false,
				},
			},
		}
	}

	if err != nil {
		response.Status = "error"
		response.Error = err.Error()
	} else {
		response.Status = "ok"
	}
	json.NewEncoder(w).Encode(response)

}

func HandleONUStatus(w http.ResponseWriter, r *http.Request) {
	CORSHeaders(w, r)
	/*
		if !CheckAuth(w, r) {
			log.Error("API command not authorized")
			return
		}
	*/
	requestvars := r.URL.Query()

	accountcode := mux.Vars(r)["accountcode"]
	subscribecode := mux.Vars(r)["subscribecode"]
	var user string

	if val, ok := requestvars["user"]; ok {
		user = val[0]
	} else {
		user = "unknown"
	}
	var response Response
	//	var subscribe maxbill.Subscribe
	var err error

	if subscribecode != "" && accountcode != "" {
		devicename := accountcode + "-" + subscribecode + "-ONT"
		var commandresult mcp.UICommand
		commandresult, err = mcp.UIRunCommand("adtn_1u_olt/onu/status/device", "device", devicename)
		//subscribe, err = maxbill.GetSubscribe(CoreDB, accountcode, subscribecode)
		log.Infof("Running ONU Status on %v - %v %v", accountcode, subscribecode, commandresult)
		if commandresult.Message != "" {
			response.Error = commandresult.Message
			response.Status = "error"
		} else {
			resultData := TestResults{
				TestStart:   time.Now(),
				RequestUser: user,
				TestName:    "ONU Status from MCP",
			}
			for _, row := range commandresult.Table.Rows {
				log.Debugf("result row is %v", row)
				resultData.Results = append(resultData.Results, TestResult{
					Name:         row.Cells[0].Data,
					ResultString: row.Cells[1].Data,
					Pass:         true,
				})
			}

			response.Data = resultData
		}

	} else {
		err = errors.New("You must supply an accountcode and a subscribe code!")
	}
	if err != nil {
		response.Status = "error"
		response.Error = err.Error()
	} else {
		response.Status = "ok"
	}
	log.Debugf("Test result is %v", response)
	json.NewEncoder(w).Encode(response)

}
