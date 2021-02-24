//
// Provisioning engine for netconf devices
//

package main

import (
	"encoding/json"
	//"encoding/xml"
	"flag"
	//"go.mongodb.org/mongo-driver/mongo"
	//	"go.mongodb.org/mongo-driver/mongo/options"
	"context"
	"net/http"
	"time"
	//"fmt"
	jnetconf "github.com/Juniper/go-netconf/netconf"
	//	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	//"go.mongodb.org/mongo-driver/bson"
	"golang.org/x/crypto/ssh"
	//"tm-provision/dhcpdb"
	"tm-provision/netconf"
	//"strings"
)

var (
	LogLevel  = flag.String("loglevel", "info", "Log level")
	Listen    = flag.String("listen", ":8080", "HTTP API listen address:port")
	SSHConfig *ssh.ClientConfig
)

func main() {

	//func init() {
	flag.Parse()
	lvl, _ := log.ParseLevel(*LogLevel)
	log.SetLevel(lvl)

	Configuration := netconf.LoadConfig()
	SSHConfig = jnetconf.SSHConfigPassword(Configuration.Username, Configuration.Password)
	Database = *CoredbConnect()
	//	dhcpdb.SQLConnect(&dhcpd.SQL)

	http.HandleFunc("/status", GetCircuitStatus)
	http.HandleFunc("/provision", ProvisionPort)
	http.HandleFunc("/unprovision", UnProvisionPort)

	log.Fatal(http.ListenAndServe(*Listen, nil))

	//}

	//	r := netconf.HostCommandRaw(s, "<get-system-information/>")

	//	r := netconf.HostCommandRaw(s, "<load-configuration><configuration><interfaces><interface><name>ge-0/0/0</name><description>Test</description></interface></interfaces></configuration></load-configuration>")
	//r := netconf.HostCommandRaw(s, "<get-configuration><configuration><interfaces><interface><name>ge-0/0/0</name></interface></interfaces></configuration></get-configuration>")
	//	log.Info(string(r))
	//	r = netconf.HostCommandRaw(s, "<commit-configuration/>")
	/*
		section := netconf.JuniperConfig{
			Interfaces: []netconf.Interfaces{
				netconf.Interfaces{
					PhysicalInterface: []netconf.PhysicalInterface{
						netconf.PhysicalInterface{
							Name: "ge-0/0/0",
						},
					},
				},
			},
		}
	*/
	/*
		var section netconf.JuniperConfig

		section.Interfaces = netconf.Interfaces{
			PhysicalInterface: []netconf.PhysicalInterface{
				netconf.PhysicalInterface{
					Name: "et-0/0/48",
				},
			},
		}
		config := netconf.ConfigurationGet(s, section)

		output, err := xml.MarshalIndent(config, "", "   ")
		if err != nil {
			log.Info(err)
		}
		log.Info(string(output))

		//	status := netconf.GetInterfaceStatus(s, "et-0/0/48")
		//	output, _ := xml.MarshalIndent(status, "", "   ")
		//	log.Info(string(output))
	*/
}

func UnProvisionPort(w http.ResponseWriter, r *http.Request) {
	keys, ok := r.URL.Query()["circuit_id"]

	if !ok || len(keys[0]) < 1 {
		http.Error(w, "Invalid Request key", 404)
		log.Info("Did not receive a circuit_id field in request")
		// return error
		return
	}
	circuitID := keys[0]
	log.Info("Getting provisioning data for circuit ID " + circuitID)

	circuitData := GetaccessPortbyID(circuitID)
	var (
		success netconf.ConfigureStatus
		//ctx     *context.Context
		//		cancel  context.CancelFunc
	)
	if circuitData.NetworkElement == "" {
		success.ErrorMessages = append(success.ErrorMessages, "No network element")
		success.Success = false

	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		log.Info(circuitData.NetworkElement)
		log.Info(circuitData.AccessData.SwitchPort)

		switch circuitData.AccessData.Platform {
		case "juniper-qfx":
			success = UnProvisionQFX(ctx, circuitData)

			//	case "adtran-sdx":

			//	}
		default:
			success.ErrorMessages = append(success.ErrorMessages, "Platform type "+circuitData.AccessData.Platform+" has not been implemented!")
			success.Success = false
		}
	}
	w.Header().Set("Content-Type", "application/json")
	//	content := gin.H{"status": "OK"}
	jsonresp, _ := json.Marshal(success)
	w.Write(jsonresp)
}

func ProvisionPort(w http.ResponseWriter, r *http.Request) {
	keys, ok := r.URL.Query()["circuit_id"]

	if !ok || len(keys[0]) < 1 {
		http.Error(w, "Invalid Request key", 404)
		log.Info("Did not receive a circuit_id field in request")
		// return error
		return
	}
	circuitID := keys[0]
	log.Info("Getting provisioning data for circuit ID " + circuitID)

	circuitData := GetaccessPortbyID(circuitID)
	var (
		success netconf.ConfigureStatus
		//ctx     *context.Context
		//		cancel  context.CancelFunc
	)

	if circuitData.Status == "Available" {
		success.ErrorMessages = append(success.ErrorMessages, "Circuit not assigned")
		success.Success = false

	} else if circuitData.NetworkElement == "" {
		success.ErrorMessages = append(success.ErrorMessages, "No network element")
		success.Success = false

	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		log.Info(circuitData.NetworkElement)
		log.Info(circuitData.AccessData.SwitchPort)

		subscribeData := GetaccessProduct(circuitData.CustomerCode, circuitData.SubscribeCode)

		switch circuitData.AccessData.Platform {
		case "juniper-qfx":
			success = ProvisionQFX(ctx, circuitData, subscribeData)

			//	case "adtran-sdx":

			//	}
		default:
			success.ErrorMessages = append(success.ErrorMessages, "Platform type "+circuitData.AccessData.Platform+" has not been implemented!")
			success.Success = false
		}
	}
	w.Header().Set("Content-Type", "application/json")
	//	content := gin.H{"status": "OK"}
	jsonresp, _ := json.Marshal(success)
	w.Write(jsonresp)
}

func GetCircuitStatus(w http.ResponseWriter, r *http.Request) {
	keys, ok := r.URL.Query()["circuit_id"]

	if !ok || len(keys[0]) < 1 {
		http.Error(w, "Invalid Request key", 404)
		log.Info("Did not receive a circuit_id field in request")
		// return error
		return
	}
	circuitID := keys[0]
	log.Info("Requesting status for circuit ID " + circuitID)

	result := GetaccessPortbyID(circuitID)

	if result.NetworkElement != "" {
		log.Info(result.NetworkElement)
		portData := result.AccessData

		s := netconf.HostConnect(result.NetworkElement, SSHConfig)
		defer s.Close()
		content, err := netconf.GetInterfaceStatus(s, portData.SwitchPort)
		if err != nil {
			http.Error(w, err.Error(), 500)
		} else {
			w.Header().Set("Content-Type", "application/json")
			//	content := gin.H{"status": "OK"}
			jsonresp, _ := json.Marshal(content)
			w.Write(jsonresp)
		}
	} else {
		http.Error(w, "Circuit ID not found!", 410)
		log.Info("Did not find data for circuit_id " + circuitID)
	}
}
