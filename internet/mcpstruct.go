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
