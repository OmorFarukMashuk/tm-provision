package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	QGISAPI    = "http://qgis.api.telmax.ca:5008/"
	QGISAPIKey = "098g5467o456n78s8e5878c8ty4578uihdsrc"
)

type Site struct {
	ID          int
	DA          string
	WireCentre  string
	FullAddress string
	AccessType  string
	Status      string
	CircuitData []struct {
		Fibre         int
		FDH           string
		Terminal      string
		FDHPort       int
		PON           string
		DemarcPort    int
		InitialSignal float32
	}
}

func GetSite(ID string) (site Site, err error) {
	client := http.Client{
		Timeout: time.Second * 4,
	}
	url := QGISAPI + "/getsite/" + ID
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("api-key", QGISAPIKey)
	var response *http.Response
	response, err = client.Do(req)
	if err != nil {
		log.Errorf("Problem fetching from QGIS API %v", err)
		return
	}
	defer response.Body.Close()
	var result []byte
	result, err = ioutil.ReadAll(response.Body)
	//log.Infof("QGIS result is %v", string(result))
	if err != nil {
		log.Errorf("Problem Reading HTTP Response %v", err)
		return
	} else {
		var resultObj struct {
			Status string `json:"status"`
			Data   Site   `json:"data"`
			Error  string `json:"error"`
		}
		err = json.Unmarshal(result, &resultObj)
		if err != nil {
			log.Errorf("Problem unmarshalling site result %v", err)
		}
		if resultObj.Status != "ok" {
			log.Errorf("Problem getting site %v", resultObj.Error)
			err = errors.New(resultObj.Error)
		} else {
			site = resultObj.Data
		}
	}
	return

}
