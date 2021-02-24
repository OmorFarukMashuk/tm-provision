// Go NETCONF Client - Juniper Example (show system information)
//
// Copyright (c) 2013-2018, Juniper Networks, Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// main package re-written for telMAX by Tim St. Pierre

package netconf

import (
	//	"bufio"
	"encoding/xml"
	//"flag"
	"fmt"
	//"log"
	log "github.com/sirupsen/logrus"

	//"os"
	//	"syscall"
	jnetconf "github.com/Juniper/go-netconf/netconf"
	"github.com/tkanos/gonfig"
	"golang.org/x/crypto/ssh"
	//"io/ioutil"
	//	"golang.org/x/crypto/ssh/terminal"
	"errors"
	//"github.com/davecgh/go-spew/spew"
	"strings"
)

type Configuration struct {
	Username string
	Keyfile  string
	Password string
}

func LoadConfig() Configuration {
	filename := "netconf.cfg"
	configuration := Configuration{}
	err := gonfig.GetConf(filename, &configuration)
	if err != nil {
		log.Fatal(fmt.Errorf("Error loading Configuration File %s", err))
	}
	log.Info("Connecting with username " + configuration.Username)
	return configuration
}

func HostConnect(host string, c *ssh.ClientConfig) *jnetconf.Session {
	s, err := jnetconf.DialSSH(host, c)
	if err != nil {
		log.Fatal(fmt.Errorf("Error connecting to host &s", err))
	}

	return s
}

func HostCommandRaw(s *jnetconf.Session, c string) string {
	log.Debug("Sending command to network device\n" + c)

	reply, err := s.Exec(jnetconf.RawMethod(c))
	if err != nil {
		log.Fatal(fmt.Errorf("error executing command %s", err))
	}
	log.Debug("Raw reply is \n" + reply.RawReply)
	return reply.Data

}

func ConfigurationGet(s *jnetconf.Session, section *JuniperConfig) (output *JuniperConfig) {
	request, err := xml.MarshalIndent(section, "", "   ")

	requeststr := "<get-configuration>" + string(request) + "</get-configuration>"

	log.Debug(requeststr)

	reply, err := s.Exec(jnetconf.RawMethod(requeststr))
	if err != nil {
		log.Fatal(err)
	}
	log.Info(reply.Data)
	//replyxml := strings.Replace(reply.Data, "\n", "", -1)

	err = xml.Unmarshal([]byte(reply.Data), &output)
	if err != nil {
		log.Fatal(err)
	}

	return output
}

func ConfigurationPut(s *jnetconf.Session, c *JuniperConfig) ConfigureStatus {
	var result ConfigurationResults
	var status ConfigureStatus
	var commitresult CommitResults

	request, err := xml.MarshalIndent(&c, "", "   ")
	log.Debug(err)
	requeststr := "<load-configuration action=\"replace\">" + string(request) + "</load-configuration>"
	log.Debug(requeststr)

	reply, err := s.Exec(jnetconf.RawMethod(requeststr))
	if err != nil {
		log.Fatal(err)
	}
	log.Debug(reply.Data)

	err = xml.Unmarshal([]byte(strings.Replace(reply.RawReply, "\n", "", -1)), &result)
	if err != nil {
		log.Fatal(err)
	}

	//log.Info(result.ErrorCount)

	if result.ErrorCount > 0 {
		log.Info("Configuration not accepted!")
		for _, error := range result.ErrorMessages {

			log.Info(error.ErrorMessage)

			status.ErrorMessages = append(status.ErrorMessages, error.ErrorMessage)
		}
		status.Success = false
		return status

	} else {

		requeststr = "<commit/>"
		reply, err = s.Exec(jnetconf.RawMethod(requeststr))
		if err != nil {
			log.Fatal(err)
		}
		log.Debug(reply.Data)

		err = xml.Unmarshal([]byte(strings.Replace(reply.RawReply, "\n", "", -1)), &commitresult)
		if err != nil {
			log.Fatal(err)
		}
		if len(commitresult.ErrorMessages) > 0 {
			log.Info("Commit Errors")
			for _, error := range commitresult.ErrorMessages {

				log.Info(error.ErrorMessage)

				status.ErrorMessages = append(status.ErrorMessages, error.ErrorMessage)
			}
			status.Success = false
		} else {
			status.Success = true
		}
	}
	//log.Info(spew.Sdump(status))

	return status
}

func GetInterfaceStatus(s *jnetconf.Session, n string) (o *InterfaceStatus, err error) {
	log.Debug("Getting status for interface " + n)

	reply, err := s.Exec(jnetconf.RawMethod("<get-interface-information><interface-name>" + n + "</interface-name></get-interface-information>"))
	if err != nil {
		log.Fatal(fmt.Errorf("Error getting interface status %s", err))
	}
	log.Debug(reply)
	replyxml := strings.Replace(reply.RawReply, "\n", "", -1)

	err = xml.Unmarshal([]byte(replyxml), &o)
	if err != nil {
		log.Fatal(err)
	}
	//	if reply.Errors != nil {
	if o.Result != "" {
		log.Info("RPC Error")
		//log.Info(spew.Sdump(reply))
		log.Info(o.Result)
		errors.New(o.Result)
		return o, nil
	} else {
		//		log.Info(reply.Errors)
		//		log.Info(reply.Errors.Message)
		log.Info("RPC Reply is OK")
		o.Result = "Success"

		reply, err = s.Exec(jnetconf.RawMethod("<get-interface-optics-diagnostics-information><interface-name>" + n + "</interface-name></get-interface-optics-diagnostics-information>"))
		if err != nil {
			log.Fatal(fmt.Errorf("Error getting interface diagnostics %s", err))
		}
		log.Debug(reply)
		replyxml = strings.Replace(reply.RawReply, "\n", "", -1)

		//	status := *InterfaceStatus.SignalLevel

		err = xml.Unmarshal([]byte(replyxml), &o.SignalLevel)
		if err != nil {
			log.Fatal(err)
		}
		return o, nil
	}
}
