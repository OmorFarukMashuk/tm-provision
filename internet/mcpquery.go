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
		log.Debugf("Authorization response was %v", string(result))
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
				log.Debugf("Auth token is %v", token)
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
		err = json.Unmarshal(result, &mcpresponse)
		if err != nil {
			log.Errorf("Problem unmarshalling MCP response %v", err)
		}
	}
	mcpresponse.Output.FixTime()
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
	req, err = http.NewRequest(http.MethodGet, url, nil)
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

func MCPGetTransaction(token string, id string) (transaction MCPTransResult, err error) {
	query := "adtran-cloud-platform-uiworkflow:transitions/transition=" + id
	var result []byte
	result, err = MCPQuery(token, query)
	if err != nil {
		log.Errorf("Problem getting transaction %v - %v", id, err)
		return
	}
	err = json.Unmarshal(result, &transaction)
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
	log.Infof("Device info is %v", deviceInfo)
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

	// Create FXS Interfaces
	for index := 0; index < int(ONT.Definition.PotsPorts); index++ {
		var iface MCPInterface
		iface.InterfaceContext.InterfaceName = subscriber + "-fxs" + strconv.Itoa(index+1)
		iface.InterfaceContext.InterfaceType = "fxs"
		iface.InterfaceContext.DeviceName = subscriber + "-ONT"
		iface.InterfaceContext.InterfaceID = "fxs 0/" + strconv.Itoa(index+1)
		iface.InterfaceContext.ProfileVector = "FXS Interface Profile Vector"
		mcpresult, err = MCPRequest(token, "adtran-cloud-platform-orchestration:create", iface)
		log.Infof("MCP result is %v", mcpresult)
		time.Sleep(time.Second * 3)
	}
	return err

}

// Modify ONT Parameters
func UpdateONT(subscriber string, ONT ONTData) error {
	token, err := MCPAuth()
	if err != nil {
		log.Errorf("Could not authenticate to MCP %v", err)
		return err
	}

	if err != nil {
		log.Infof("Problem authenticating to MCP %v", err)
		return err
	}
	var device MCPDevice
	device.DeviceContext.DeviceName = subscriber + "-ONT"

	/* Not sure if we need this
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
	log.Infof("Device info is %v", deviceInfo)
	log.Errorf("Checked for existing device - error was %v", err)
	*/
	device.DeviceContext.ModelName = ONT.Definition.Model
	device.DeviceContext.ProfileVector = "ONU Config Vector"
	var emptystruct struct{}
	device.DeviceContext.ManagementDomainContext.ManagementDomainExternal = emptystruct
	var mcpresult MCPResult

	device.DeviceContext.ObjectParameters.Serial = ONT.Device.Serial
	//		device.DeviceContext.ObjectParameters.OnuID = ONU
	//		device.DeviceContext.UpstreamInterface = PON
	mcpresult, err = MCPRequest(token, "adtran-cloud-platform-orchestration:modify", device)
	log.Infof("MCP result is %v", mcpresult)
	time.Sleep(time.Second * 5)

	if err != nil {
		return err
	}
	log.Infof("MCP result is %v", mcpresult.Output.Status)
	err = ReflowDevice(token, []string{device.DeviceContext.DeviceName}, "API Reflow ONT")
	if err != nil {
		log.Errorf("Problem with device reflow %v %v", mcpresult.Errors, err)
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
		if int(serviceInfo.Uplink.InterfaceEndpoint.OuterTagVlanID.(float64)) != vlan {
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

// Create a voice service
func CreatePhoneService(name string, device string, subscriberid string, profile string, contentprovider string, vlan int, number string, password string, port int) error {
	log.Infof("Adding phone service to %v on port %v", device, port)
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
		if int(serviceInfo.Uplink.InterfaceEndpoint.OuterTagVlanID.(float64)) != vlan {
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
	service.ServiceContext.ServiceType = "voice-sip-service"

	service.ServiceContext.UplinkContext.InterfaceEndpoint.OuterTagVlanID = vlan
	service.ServiceContext.UplinkContext.InterfaceEndpoint.InnerTagVlanID = "none"
	service.ServiceContext.UplinkContext.InterfaceEndpoint.ContentProviderName = contentprovider
	service.ServiceContext.DownlinkContext.InterfaceEndpoint.OuterTagVlanID = "untagged"
	service.ServiceContext.DownlinkContext.InterfaceEndpoint.InnerTagVlanID = "none"
	service.ServiceContext.DownlinkContext.InterfaceEndpoint.InterfaceName = subscriberid + "-fxs" + strconv.Itoa(port)
	service.ServiceContext.ObjectParameters.SIPIdentity = number
	service.ServiceContext.ObjectParameters.SIPUser = number
	service.ServiceContext.ObjectParameters.SIPPassword = password

	var mcpresult MCPResult
	log.Infof("Service data for phone is %v", service)
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

func ReflowDevice(token string, devices []string, jobname string) error {
	var query string
	var data MCPJob
	var mcpresult MCPResult
	var err error
	var transresult MCPTransResult
	data.JobContext.JobName = jobname

	var jobdata struct {
		JobContext struct {
			JobName string `json:"job-name"`
		} `json:"job-context"`
	}

	var rundata struct {
		JobName string `json:"job-name"`
	}

	jobdata.JobContext.JobName = jobname
	rundata.JobName = jobname
	/*
		log.Infof("Deactivating %v job", jobname)
		query = "adtran-cloud-platform-uiworkflow:deactivate"
		mcpresult, err = MCPRequest(token, query, jobdata)
		if err != nil {
			log.Errorf("Problem deactivating old reflow job %v", err)
			//		return err
		}
		log.Infof("MCP result is %v", mcpresult)
		time.Sleep(time.Second * 5)
	*/
	log.Infof("Undeploying %v job", jobname)
	query = "adtran-cloud-platform-uiworkflow:undeploy"
	mcpresult, err = MCPRequest(token, query, data)
	if err != nil {
		log.Errorf("Problem un-deploying reflow job %v", err)
		//		return err
	}
	log.Infof("MCP result is %v", mcpresult)
	time.Sleep(time.Second * 3)

	log.Infof("Re-deploying Re-flow job %v with devices %v", jobname, devices)
	query = "adtran-cloud-platform-uiworkflow:deploy"
	data.PopulateDevice(devices)
	mcpresult, err = MCPRequest(token, query, data)
	if err != nil {
		log.Errorf("Problem deploying reflow job %v", err)
		return err
	}
	log.Infof("MCP result from job deploy was %v", mcpresult)
	time.Sleep(time.Second * 3)

	log.Infof("Transaction id was %v", mcpresult.Output.TransID)
	transresult, err = MCPGetTransaction(token, mcpresult.Output.TransID)
	log.Infof("Reflow result was %v", transresult)
	if transresult.Completion != "completed-ok" {
		err = errors.New("Reflow failed - " + transresult.Status)
		return err
	}
	/*
		query = "adtran-cloud-platform-uiworkflow:activate"
		mcpresult, err = MCPRequest(token, query, data)
		time.Sleep(time.Second * 5)

		log.Infof("Transaction id was %v", mcpresult.Output.TransID)
		transresult, err = MCPGetTransaction(token, mcpresult.Output.TransID)
		log.Infof("Reflow run result was %v", transresult)
		if transresult.Completion != "completed-ok" {
			err = errors.New("Reflow failed - " + transresult.Status)
		}
	*/
	query = "adtran-cloud-platform-uiworkflow-jobs:run-job-now"
	mcpresult, err = MCPRequest(token, query, rundata)
	if err != nil {
		log.Errorf("Problem running reflow job %v", err)
		return err
	}
	/*
		time.Sleep(time.Second * 10)

		log.Infof("Transaction id was %v", mcpresult.Output.TransID)
		transresult, err = MCPGetTransaction(token, mcpresult.Output.TransID)
		log.Infof("Reflow result was %v", transresult)
		if transresult.Completion != "completed-ok" {
			err = errors.New("Reflow failed - " + transresult.Status)
			return err
		}
	*/
	return err
}
