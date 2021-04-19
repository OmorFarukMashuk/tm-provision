package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"time"
)

var (
	MCPURL = flag.String("mcpurl", "https://mcp01.dc1.osh.telmax.ca/api/restconf/operations/", "URL and prefix for MCP Interaction")
)

func MCPAuth(username string, password string) (token string, err error) {
	Client := http.Client{
		Timeout: time.Second * 4,
	}
	url := *MCPURL + "adtran-auth-toker:request-token"
	var req *http.Request
	var jsonStr []byte
	var authData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	jsonStr, err = json.Marshal(authData)
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
		var responseData struct {
			Token   string `json:"token"`
			Message string `json:"message"`
		}
		err = json.Unmarshal(result, &responseData)
		if err != nil {
			if token != "" {
				token = responseData.Token
			} else {
				err = errors.New(responseData.Message)
				log.Errorf("Problem authorizing with MCP - %v", responseData.Message)
			}
		}
	}
	return
}

func MCPRequest(authtoken, string, command string, data interface{}) (responseObj interface{}, err error) {
	Client := http.Client{
		Timeout: time.Second * 4,
	}
	url := *MCPURL + command
	var req *http.Request
	var jsonStr []byte
	jsonStr, err = json.Marshal(data)
	log.Debugf("Posted string is %v", jsonStr)
	if err != nil {
		log.Errorf("Problem marshalling JSON data", err)
		return
	}
	req, err = http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonStr))
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
		json.Unmarshal(result, &responseObj)
	}
	return
}
