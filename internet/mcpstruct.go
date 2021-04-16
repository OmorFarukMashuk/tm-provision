package main

import ()

var ()

type MCPDeviceCreate struct {
	DeviceContext struct {
		DeviceName              string `json:"device-name"`
		ModelName               string `json:"model-name"`
		ManagementDomainContext struct {
			ManagementDomainExternal interface{} `json:"management-domain-external"`
		} `json:"management-domain-context,omitempty"`
	} `json:"device-context"`
	ProfileVector     string `json:"profile-vector-name"`
	BaseConfiguration string `json:"base-configuration"`
}

type MCPDeviceDeploy struct {
	DeviceContext struct {
		DeviceName       string `json:"device-name"`
		ObjectParameters struct {
			Serial string `json:"serial-number"`
			OnuID  int    `json:"onu-id"`
		} `json:"object-parameters"`
		InterfaceName string `json:"interface-name"`
	}
}

type MCPDeviceActivate struct {
	DeviceContext struct {
		DeviceName string `json:"device-name"`
	}
}




type MCPInterfaceCreate struct {
	InterfaceContext struct {
		InterfaceName              string `json:"interface-name"`
		InterfaceType               string `json:"interface-type"`
	} `json:"device-context"`
	ProfileVector     string `json:"profile-vector-name"`
}

type MCPInterfaceDeploy struct {
	InterfaceContext struct {
		InterfaceName       string `json:"interface-name"`
		InterfaceType       string `json:"interface-type"`
		DeviceName string `json:"device-name"`
		InterfaceID       string `json:"interface-id"`
	}
}

type MCPInterfaceActivate struct {
	InterfaceContext struct {
		InterfaceName string `json:"interface-name"`
	}
}







type MCPServiceCreate struct {
	ServiceContext struct {
		ServiceID              string `json:"service-id"`
		ServiceType            string `json:"service-type"`
		RemoteID               string `json:"remote-id"`
		CircuitID              string `json:"circuit-id"`
	} `json:"device-context"`

	ProfileName     string `json:"profile-name"`
}

type MCPServiceDeploy struct {
	ServiceContext struct {
		ServiceID       string `json:"service-name"`

		UplinkContext struct {
			InterfaceEndpoint struct {
				OuterTagVlanID string `json:"outer-tag-vlan-id"`
				InnerTagVlanID string `json:"inner-tag-vlan-id"`
				ContentProviderName string `json:"content-provider-name"`
			}
		} `json:"uplink-context"`


		DownlinkContext struct {
			InterfaceEndpoint struct {
				OuterTagVlanID string `json:"outer-tag-vlan-id"`
				InnerTagVlanID string `json:"inner-tag-vlan-id"`
				InterfaceName string `json:"interface-name"`
			}
		} `json:"downlink-context"`

	}
}

type MCPServiceActivate struct {
	ServiceContext struct {
		ServiceID string `json:"service-id"`
	}
}



