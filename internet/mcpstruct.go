package main

import (
	"bitbucket.org/timstpierre/telmax-common"
	"time"
)

var ()

type ONTData struct {
	Device     telmax.Device
	Definition telmax.DeviceDefinition
}

type MCPDevice struct {
	DeviceContext struct {
		DeviceName       string `json:"device-name"`
		ModelName        string `json:"model-name,omitempty"`
		ObjectParameters struct {
			Serial string `json:"serial-number,omitempty"`
			OnuID  int    `json:"onu-id,omitempty"`
		} `json:"object-parameters,omitempty"`
		ManagementDomainContext struct {
			ManagementDomainExternal interface{} `json:"management-domain-external"`
		} `json:"management-domain-context,omitempty"`
		ProfileVector     string `json:"profile-vector-name,omitempty"`
		BaseConfiguration string `json:"base-configuration"`
		UpstreamInterface string `json:"interface-name,omitempty"`
	} `json:"device-context"`
}

type MCPInterface struct {
	InterfaceContext struct {
		InterfaceName string `json:"interface-name"`
		InterfaceType string `json:"interface-type,omitempty"`
		DeviceName    string `json:"device-name,omitempty"`
		InterfaceID   string `json:"interface-id,omitempty"`
		ProfileVector string `json:"profile-vector-name,omitempty"`
	} `json:"interface-context"`
}

type MCPService struct {
	ServiceContext struct {
		ServiceID     string `json:"service-id"`
		ServiceType   string `json:"service-type"`
		RemoteID      string `json:"remote-id"`
		CircuitID     string `json:"circuit-id"`
		UplinkContext struct {
			InterfaceEndpoint struct {
				OuterTagVlanID      string `json:"outer-tag-vlan-id"`
				InnerTagVlanID      string `json:"inner-tag-vlan-id"`
				ContentProviderName string `json:"content-provider-name"`
			}
		} `json:"uplink-context,omitempty"`

		DownlinkContext struct {
			InterfaceEndpoint struct {
				OuterTagVlanID string `json:"outer-tag-vlan-id"`
				InnerTagVlanID string `json:"inner-tag-vlan-id"`
				InterfaceName  string `json:"interface-name"`
			}
		} `json:"downlink-context,omitempty"`
	} `json:"service-context"`

	ProfileName string `json:"profile-name,omitempty"`
}

type MCPResult struct {
	Errors struct {
		Type    string `json:"error-type"`
		Tag     string `json:"error-tag"`
		Message string `json:"error-message"`
	} `json:"errors"`
	Output struct {
		DeviceName string    `json:"device-name"`
		TimeStamp  time.Time `json:"timestamp"`
		Status     string    `json:"status"`
		TransID    string    `json:"trans-id"`
		Completion string    `json:"completion-status"`
	}
}
