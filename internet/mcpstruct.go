package main

import ()

var ()

type MCPRequest struct {
	Field  string   `json:"field"`
	Field2 []string `json:"field2,omitempty"`
	Field3 MCPDevice
}

type MCPDevice struct {
	Model
	Interfaces []strings
}
