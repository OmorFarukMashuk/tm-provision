//
// Service Provisioning functions for Juniper QFX devices
//

package main

import (
	//"encoding/json"
	"encoding/xml"
	//"go.mongodb.org/mongo-driver/mongo"
	//	"go.mongodb.org/mongo-driver/mongo/options"
	"context"
	//"net/http"

	//"fmt"
	//jnetconf "github.com/Juniper/go-netconf/netconf"
	//	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	//"go.mongodb.org/mongo-driver/bson"
	//"golang.org/x/crypto/ssh"
	//"strings"
	"github.com/davecgh/go-spew/spew"
	"strconv"
	"tm-provision/dhcpdb"
	"tm-provision/netconf"
)

// Un-provision QFX customer facing port.
//
func UnProvisionQFX(ctx context.Context, circuitData accessPort) netconf.ConfigureStatus {
	log.Info("Setting empty provisioning for QFX")
	portData := circuitData.AccessData
	var status netconf.ConfigureStatus

	_ = dhcpdb.DhcpReleaseAll(ctx, circuitData.CircuitID)

	configuration := netconf.JuniperConfig{
		Interfaces: netconf.Interfaces{

			PhysicalInterface: []netconf.PhysicalInterface{
				netconf.PhysicalInterface{
					MergeMode:   "replace",
					Name:        portData.SwitchPort,
					Description: circuitData.CircuitID + " | Unprovisioned",
				},
			},
		},
	}
	xmlstr, err := xml.MarshalIndent(&configuration, "", "   ")
	if err != nil {
		log.Debug(err)
	}
	log.Info(string(xmlstr))
	s := netconf.HostConnect(circuitData.NetworkElement, SSHConfig)
	defer s.Close()

	status = netconf.ConfigurationPut(s, &configuration)
	if status.Success {
		UpdateaccessPortStatus(circuitData.CircuitID, "Available")
	}
	return status

}

// Provision a QFX switch as an access device.  Takes circuit and subscription data, returns configuration status object.
//
func ProvisionQFX(ctx context.Context, circuitData accessPort, subscribeData []productData) netconf.ConfigureStatus {
	log.Info("Creating provisioning for QFX")
	portData := circuitData.AccessData

	//unit := 0
	//	var product productData

	//	var luns []netconf.LogicalInterface
	lun := make(map[int]netconf.LogicalInterface)

	var status netconf.ConfigureStatus
	var nativevlan int
	var flexvlan string
	flexvlan = " "

	for _, product := range subscribeData {
		if product.NetworkProfile != nil {
			log.Info("Product Data: " + product.ProductName + " Category:" + product.Category)
			log.Info("Network Profile name " + product.NetworkProfile.Name)

			// Switch based on the product category
			switch product.Category {

			case "Internet":
				var (
					internet netconf.LogicalInterface
					inetvlan int
				)
				profile := product.NetworkProfile
				log.Info(spew.Sdump(profile))

				switch profile.RoutingMode {
				// This is a layer 3 hand-off, so we need to configure the interface as a routing interface with an address.
				case "l3-handoff":
					internet = netconf.LogicalInterface{
						//Unit:        0,
						Description: "Internet Access",
						Inet: &netconf.LogicalIPAddr{
							IPAddress: "10.0.0.1/24",
						},
						Inet6: &netconf.LogicalIPAddr{
							IPAddress: "2600:db8::/56",
						},
					}
					lun[0] = internet

				case "bridged":
					// This is a shared subnet Internet connection

					if profile.Vlan != 0 {
						// If the profile has a vlan associated, use that to provision.
						log.Info("Using profile supplied vlan ID " + strconv.FormatInt(int64(profile.Vlan), 10))
						inetvlan = profile.Vlan

					} else if profile.AddressPool != "" {
						// If the profile uses an address pool, then allocate an address from that pool and get the vlan.
						var dhcpid string
						dhcpid = circuitData.NetworkElement + "-" + portData.SwitchPort
						success, addr_alloc := dhcpdb.DhcpAssign(ctx, profile.AddressPool, circuitData.CustomerCode, circuitData.SubscribeCode, circuitData.CircuitID, dhcpid)
						if !success {
							log.Fatal("Could not allocate address!")
						} else {
							inetvlan = addr_alloc.VlanID
						}
					}
					// Create a logical interface that makes the Internet vlan native / untagged
					internet = netconf.LogicalInterface{
						//Unit:          0,
						Description:   "Bridged Internet Access",
						Encapsulation: "vlan-bridge",
						VlanID:        inetvlan,
						/*
							Switching: &netconf.EthernetSwitching{
								PortMode: "access",
								VlanMembers: []netconf.VlanMembers{
									netconf.VlanMembers{
										Member: profile.Vlan,
									},
								},
							},
						*/
					}
					nativevlan = profile.Vlan
					flexvlan = " "
					/*
						if profile.InputFilter != "" {
							internet.Switching.InputFilter = profile.InputFilter
						}
						if profile.OutputFilter != "" {
							internet.Switching.OutputFilter = profile.OutputFilter
						}
					*/
					lun[0] = internet
				}

			case "TV":
				var tv netconf.LogicalInterface
				profile := product.NetworkProfile
				tv = netconf.LogicalInterface{
					Description:   "Bridged TV",
					VlanID:        200,
					Encapsulation: "vlan-bridge",
				}
				if profile.InputFilter != "" {
					tv.InputFilter = profile.InputFilter
				}
				if profile.OutputFilter != "" {
					tv.OutputFilter = profile.OutputFilter
				}
				lun[10] = tv

			}
		}

	}

	var luns []netconf.LogicalInterface
	for unit, lundata := range lun {
		lundata.Unit = unit
		luns = append(luns, lundata)
	}
	configuration := netconf.JuniperConfig{
		Interfaces: netconf.Interfaces{

			PhysicalInterface: []netconf.PhysicalInterface{
				netconf.PhysicalInterface{
					MergeMode:   "replace",
					Name:        portData.SwitchPort,
					Description: circuitData.CircuitID + "| Cust " + circuitData.CustomerCode + "| Subs " + circuitData.SubscribeCode,
					//					Encapsulation: "flexible-ethernet-services",
					LogicalUnits:  luns,
					NativeVlanID:  nativevlan,
					FlexVlan:      flexvlan,
					Encapsulation: "flexible-ethernet-services",
				},
			},
		},
	}
	xmlstr, err := xml.MarshalIndent(&configuration, "", "   ")
	if err != nil {
		log.Debug(err)
	}
	log.Info(string(xmlstr))
	s := netconf.HostConnect(circuitData.NetworkElement, SSHConfig)
	defer s.Close()

	status = netconf.ConfigurationPut(s, &configuration)
	if status.Success {
		UpdateaccessPortStatus(circuitData.CircuitID, "Provisioned")
	}
	return status

}
