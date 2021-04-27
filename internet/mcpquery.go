package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

var (
	MCPURL      = flag.String("mcpurl", "https://mcp01.dc1.osh.telmax.ca/api/restconf/", "URL and prefix for MCP Interaction")
	MCPUsername = flag.String("mcpusername", "tim", "MCP Username")
	MCPPassword = flag.String("mcppassword", "N3wjob!", "MCP Password")
)

func MCPAuth() (token string, err error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	Client := http.Client{
		Timeout:   time.Second * 4,
		Transport: tr,
	}
	url := *MCPURL + "operations/adtran-auth-token:request-token"
	var req *http.Request
	var jsonStr []byte
	var authData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	authData.Username = *MCPUsername
	authData.Password = *MCPPassword
	jsonStr, err = json.Marshal(authData)
	log.Debugf("Posted string is %v", string(jsonStr))
	if err != nil {
		log.Errorf("Problem marshalling JSON data", err)
		return
	}
	req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	if err != nil {
		log.Errorf("Problem generating HTTP request %v", err)
		return
	}
	var response *http.Response
	response, err = Client.Do(req)
	if err != nil {
		log.Errorf("Problem with HTTP request execution %v", err)
		return
	}
	defer response.Body.Close()
	var result []byte
	result, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Errorf("Problem Reading HTTP Response %v", err)
		return
	} else {
		log.Infof("Authorization response was %v", string(result))
		var responseData struct {
			Token   string `json:"token"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(result, &responseData)
		if err != nil {
			log.Errorf("Problem unmarshalling auth request %v - %v", string(result), err)
		} else {
			token = responseData.Token
			if token != "" {
				log.Infof("Auth token is %v", token)
			} else {
				err = errors.New(responseData.Message)
				log.Errorf("Problem authorizing with MCP - %v", responseData.Message)
			}
		}
	}
	return
}

func MCPRequest(authtoken string, command string, data interface{}) (mcpresponse MCPResult, err error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	Client := http.Client{
		Timeout:   time.Second * 4,
		Transport: tr,
	}
	url := *MCPURL + "operations/" + command
	var req *http.Request
	var jsonStr []byte
	var dataObj struct {
		Input interface{} `json:"input"`
	}
	dataObj.Input = data
	jsonStr, err = json.Marshal(dataObj)
	log.Debugf("Posted string is %v", string(jsonStr))
	if err != nil {
		log.Errorf("Problem marshalling JSON data", err)
		return
	}
	req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonStr))
	if err != nil {
		log.Errorf("Problem generating HTTP request %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+authtoken)
	var response *http.Response
	response, err = Client.Do(req)
	if err != nil {
		log.Errorf("Problem with HTTP request execution %v", err)
		return
	}
	defer response.Body.Close()
	var result []byte
	result, err = ioutil.ReadAll(response.Body)
	log.Debugf("MCP response raw was %v", string(result))
	if err != nil {
		log.Errorf("Problem Reading HTTP Response %v", err)
		return
	} else {
		json.Unmarshal(result, &mcpresponse)
	}
	if mcpresponse.Errors.Message != "" {
		err = errors.New(mcpresponse.Errors.Message)
		log.Errorf("MCP error %v", mcpresponse)
	}
	return
}

func MCPQuery(authtoken string, query string) (result []byte, err error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	Client := http.Client{
		Timeout:   time.Second * 4,
		Transport: tr,
	}
	url := *MCPURL + "data/" + query
	var req *http.Request
	log.Debugf("Query string is %v", query)
	req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer([]byte(query)))
	if err != nil {
		log.Errorf("Problem generating HTTP request %v", err)
		return
	}
	req.Header.Set("Authorization", "Bearer "+authtoken)
	var response *http.Response
	response, err = Client.Do(req)
	if err != nil {
		log.Errorf("Problem with HTTP request execution %v", err)
		return
	}
	defer response.Body.Close()
	result, err = ioutil.ReadAll(response.Body)
	log.Debugf("MCP response raw was %v", string(result))
	return
}

func CreateONT(subscriber string, ONT ONTData, PON string, ONU int) error {
	token, err := MCPAuth()
	if err != nil {
		log.Errorf("Could not authenticate to MCP %v", err)
		return err
	}
	var activateNow bool
	if PON != "" {
		activateNow = true
	}
	if err != nil {
		log.Infof("Problem authenticating to MCP %v", err)
		return err
	}
	var device MCPDevice
	device.DeviceContext.DeviceName = subscriber + "-ONT"

	var deviceInfo MCPDeviceInfo
	deviceInfo, err = GetDevice(token, device.DeviceContext.DeviceName)
	if deviceInfo.State == "deployed" {
		log.Infof("Device already deployed!")
		if ONT.Device.Serial != deviceInfo.Parameters.Serial {
			log.Errorf("Device deployed with different serial!")
			err = errors.New("Subscriber " + subscriber + " ONT already deployed with serial number " + deviceInfo.Parameters.Serial)
		}
		return err
	}
	log.Errorf("Checked for existing device - error was %v", err)

	device.DeviceContext.ModelName = ONT.Definition.Model
	device.DeviceContext.ProfileVector = "ONU Config Vector"
	var emptystruct struct{}
	device.DeviceContext.ManagementDomainContext.ManagementDomainExternal = emptystruct
	var mcpresult MCPResult

	if activateNow {
		device.DeviceContext.ObjectParameters.Serial = ONT.Device.Serial
		device.DeviceContext.ObjectParameters.OnuID = ONU
		device.DeviceContext.UpstreamInterface = PON
		mcpresult, err = MCPRequest(token, "adtran-cloud-platform-orchestration:create", device)
		log.Infof("MCP result is %v", mcpresult)
		time.Sleep(time.Second * 5)

		if err != nil {
			return err
		}
		log.Infof("MCP result is %v", mcpresult.Output.Status)

	} else {
		mcpresult, err = MCPRequest(token, "adtran-cloud-platform-uiworkflow:create", device)
		log.Infof("MCP result is %v", mcpresult)
		time.Sleep(time.Second * 5)

		if err != nil {
			log.Errorf("Problem creating ONT %v", err)
			//		return err
		}
	}
	// Create Ethernet interfaces
	for index := 0; index < int(ONT.Definition.EthernetPorts); index++ {
		var iface MCPInterface
		iface.InterfaceContext.InterfaceName = subscriber + "-eth" + strconv.Itoa(index+1)
		iface.InterfaceContext.InterfaceType = "ethernet"
		iface.InterfaceContext.DeviceName = subscriber + "-ONT"
		iface.InterfaceContext.InterfaceID = "ethernet 0/" + strconv.Itoa(index+1)
		iface.InterfaceContext.ProfileVector = "ONU Eth UNI Profile Vector"
		mcpresult, err = MCPRequest(token, "adtran-cloud-platform-orchestration:create", iface)
		log.Infof("MCP result is %v", mcpresult)
		time.Sleep(time.Second * 3)

		/*
			time.Sleep(time.Second * 5)
			mcpresult, err = MCPRequest(token, "adtran-cloud-platform-uiworkflow:deploy", iface)
			log.Infof("MCP result is %v", mcpresult)
			time.Sleep(time.Second * 5)
			mcpresult, err = MCPRequest(token, "adtran-cloud-platform-uiworkflow:activate", iface)
			log.Infof("MCP result is %v", mcpresult)
		*/
	}
	return err

}

func CreateDataService(name string, device string, subscriberid string, profile string, contentprovider string, vlan int, port int) error {
	if contentprovider == "" || vlan == 0 || profile == "" {
		log.Errorf("This service is not properly configured %v", name)
		err := errors.New("Service is missing vlan or CP")
		return err
	}
	token, err := MCPAuth()
	var serviceInfo MCPServiceInfo
	serviceInfo, err = GetService(token, name)
	if serviceInfo.State == "deployed" {
		log.Info("Service is already deployed")
		if serviceInfo.Uplink.InterfaceEndpoint.OuterTagVlanID != vlan {
			log.Errorf("Service %v created with wrong vLAN", name)
			err = errors.New("Service " + name + " already created, but vLANs do not match!")
		}
		return err
	}
	var service MCPService
	service.ServiceContext.ServiceID = name
	service.ServiceContext.RemoteID = subscriberid
	service.ServiceContext.CircuitID = subscriberid
	service.ServiceContext.ProfileName = profile

	service.ServiceContext.UplinkContext.InterfaceEndpoint.OuterTagVlanID = vlan
	service.ServiceContext.UplinkContext.InterfaceEndpoint.InnerTagVlanID = "none"
	service.ServiceContext.UplinkContext.InterfaceEndpoint.ContentProviderName = contentprovider
	service.ServiceContext.DownlinkContext.InterfaceEndpoint.OuterTagVlanID = "untagged"
	service.ServiceContext.DownlinkContext.InterfaceEndpoint.InnerTagVlanID = "none"
	service.ServiceContext.DownlinkContext.InterfaceEndpoint.InterfaceName = subscriberid + "-eth" + strconv.Itoa(port)
	var mcpresult MCPResult
	mcpresult, err = MCPRequest(token, "adtran-cloud-platform-orchestration:create", service)
	log.Infof("MCP result is %v", mcpresult)
	if err != nil {
		return err
	}
	return err
}

func GetDevice(token string, name string) (data MCPDeviceInfo, err error) {
	query := "adtran-cloud-platform-uiworkflow-devices:devices/device=" + name
	var result []byte
	result, err = MCPQuery(token, query)
	if err != nil {
		log.Errorf("Problem with device query %v", err)
		return
	}
	err = json.Unmarshal(result, &data)
	if err != nil {
		log.Errorf("Problem unmarshalling query result %v", err)
	}
	return
}

func GetService(token string, name string) (data MCPServiceInfo, err error) {
	query := "adtran-cloud-platform-uiworkflow-services:services/service=" + name
	var result []byte
	result, err = MCPQuery(token, query)
	if err != nil {
		log.Errorf("Problem with service query %v", err)
		return
	}
	err = json.Unmarshal(result, &data)
	if err != nil {
		log.Errorf("Problem unmarshalling query result %v", err)
	}
	return
}
