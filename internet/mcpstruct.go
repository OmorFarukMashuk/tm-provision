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

type OLTService struct {
	Name             string
	ProductData      telmax.Product
	SubscribeProduct telmax.SubscribedProduct
	Vlan             int
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
		ServiceID        string `json:"service-id"`
		ServiceType      string `json:"service-type,omitempty"`
		RemoteID         string `json:"remote-id"`
		CircuitID        string `json:"agent-circuit-id"`
		ObjectParameters struct {
			SIPIdentity string `json:"sip-identity"`
			SIPUser     string `json:"sip-user-name"`
			SIPPassword string `json:"sip-password"`
		} `json:"object-parameters,omitempty"`
		UplinkContext struct {
			InterfaceEndpoint struct {
				OuterTagVlanID      interface{} `json:"outer-tag-vlan-id"`
				InnerTagVlanID      interface{} `json:"inner-tag-vlan-id"`
				ContentProviderName string      `json:"content-provider-name"`
			} `json:"interface-endpoint"`
		} `json:"uplink-endpoint,omitempty"`

		DownlinkContext struct {
			InterfaceEndpoint struct {
				OuterTagVlanID interface{} `json:"outer-tag-vlan-id"`
				InnerTagVlanID interface{} `json:"inner-tag-vlan-id"`
				InterfaceName  string      `json:"interface-name"`
			} `json:"interface-endpoint"`
		} `json:"downlink-endpoint,omitempty"`
		ProfileName string `json:"profile-name,omitempty"`
	} `json:"service-context"`
}

type MCPResult struct {
	Errors struct {
		Type    string `json:"error-type"`
		Tag     string `json:"error-tag"`
		Message string `json:"error-message"`
	} `json:"errors"`
	Output MCPTransResult `json:"output"`
}

type MCPTransResult struct {
	DeviceName string `json:"device-name,omitempty"`
	JobName    string `json:"job-name,omitempty"`
	RawTime    string `json:"timestamp"`
	TimeStamp  time.Time
	Status     string `json:"status"`
	TransID    string `json:"trans-id"`
	Completion string `json:"completion-status"`
}

func (transresult *MCPTransResult) FixTime() {
	transresult.TimeStamp, _ = time.Parse(transresult.RawTime, "2006-01-02T15:04:05.000000")
}

type MCPDeviceInfo struct {
	Name       string `json:"device-name"`
	State      string `json:"state"`
	Parameters struct {
		Serial string `json:"serial-number"`
		Onu    int    `json:"onu-id,string,omitempty"`
	} `json:"object-parameters"`
	PartNumber string `json:"part-number"`
	MetaData   struct {
		Inventory struct {
			Software    string `json:"software-rev"`
			Serial      string `json:"serial-num"`
			HardwareRev string `json:"hardware-rev"`
			Model       string `json:"model-name"`
			Firmware    string `json:"firmware-rev"`
		} `json:"inventory"`
		ProfileVector string `json:"profile-vector-name"`
		Model         string `json:"model-name"`
	} `json:"metadata"`
}

type MCPInterfaceInfo struct {
	DeviceName    string `json:"device-name"`
	InterfaceName string `json:"interface-name"`
	InterfaceType string `json:"interface-type"`
	State         string `json:"state"`
	InterfaceID   string `json:"interface-id,omitempty"`
	ProfileVector string `json:"profile-vector-name,omitempty"`
}

type MCPServiceInfo struct {
	State            string `json:"state"`
	Status           string `json:"status"`
	ServiceType      string `json:"service-type,omitempty"`
	ProfileName      string `json:"profile-name,omitempty"`
	ServiceID        string `json:"service-id"`
	ServiceState     string `json:"service-state"`
	CircuitID        string `json:"agent-circuit-id"`
	ObjectParameters struct {
		SIPIdentity string `json:"sip-identity"`
		SIPUser     string `json:"sip-user-name"`
		SIPPassword string `json:"sip-password"`
	} `json:"object-parameters,omitempty"`
	Uplink struct {
		InterfaceEndpoint struct {
			DeviceName          string      `json:"device-name"`
			InterfaceName       string      `json:"interface-name"`
			OuterTagVlanID      interface{} `json:"outer-tag-vlan-id"`
			InnerTagVlanID      interface{} `json:"inner-tag-vlan-id"`
			InterfaceID         string      `json:"interface-id"`
			ContentProviderName string      `json:"content-provider-name"`
		} `json:"interface-endpoint"`
	} `json:"uplink-endpoint,omitempty"`

	Downlink struct {
		InterfaceEndpoint struct {
			DeviceName     string      `json:"device-name"`
			OuterTagVlanID interface{} `json:"outer-tag-vlan-id"`
			InnerTagVlanID interface{} `json:"inner-tag-vlan-id"`
			InterfaceID    string      `json:"interface-id"`
		} `json:"interface-endpoint"`
	} `json:"downlink-endpoint,omitempty"`
}

// MCP job object for orchestration jobs
type MCPJob struct {
	JobContext struct {
		JobName        string              `json:"job-name"`
		Action         string              `json:"action,omitempty"`
		Trigger        string              `json:"trigger,omitempty"`
		ActionContext  []JobActionContext  `json:"action-context,omitempty"`
		TriggerContext []JobTriggerContext `json:"trigger-context"`
	} `json:"job-context"`
}

type JobActionContext struct {
	Name       string          `json:"name"`
	Type       string          `json:"type"`
	Hint       string          `json:"hint,omitempty"`
	FilterList []JobFilterList `json:"filter-list"`
	ValueList  []string        `json:"value-list"`
}

type JobFilterList struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Hint      string   `json:"hint,omitempty"`
	ValueList []string `json:"value-list"`
}

type JobTriggerContext struct {
}

// Pre-populate job for activation
func (job *MCPJob) PopulateDevice(device []string) {
	job.JobContext.ActionContext = []JobActionContext{
		JobActionContext{
			Name: "Filter Criteria",
			Type: "device",
			FilterList: []JobFilterList{
				JobFilterList{
					Name:      "By Name",
					Type:      "device",
					Hint:      "The name of the ONT",
					ValueList: device,
				},
			},
			ValueList: device,
		},
	}
	job.JobContext.TriggerContext = make([]JobTriggerContext, 0)
	//		JobTriggerContext{},
	//	}
}
