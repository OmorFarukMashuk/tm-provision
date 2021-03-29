package telmaxprovision

import (
	"time"
)

type ProvisionRequest struct {
	RequestID     string             // a UUID for this request
	AccountCode   string             // The account code of the customer
	AccountName   string             // A name for the customer - fullname is best
	SubscribeCode string             // The subscribe code for this physical site or subscription
	SiteID        string             // The identifier for the physical location
	SubscribeName string             // The name of the subscription
	RequestType   string             // Valid requests are New, Update, DeviceSwap, DeviceReturn, Cancel
	RequestTicket string             // The TicketID if the request came from a ticket - used to add actions to tickets.
	RequestUser   string             //  The user to notify if something went wrong (optional)
	Products      []ProvisionProduct // A list of products to provision
	Devices       []ProvisionDevice  // A list of devices to provision

}

type ProvisionProduct struct {
	SubProductCode string // The SubProductCode for this instance
	ProductCode    string // The product definition code that defines the product
	Category       string // The product category, ie. Internet, Phone, TV

}

type ProvisionDevice struct {
	DeviceCode     string // The unique identifier for the device
	DefinitionCode string // The definition code that defines the device attributes
	DeviceType     string // The type of device from the Definition Code
	Mac            string // The MAC address of the device - normalize to letters and uppercase
}

type ProvisionResult struct {
	RequestID     string    // Match up this with the request
	Reference     string    // A reference to the thing that was provisioned - device code or sub_product_code
	ReferenceType string    // The name of the field that the reference pertains to
	Success       bool      // Was it successful
	Time          time.Time // Time that provisioning completed
	Result        string    // Human readable text about what the outcome was
}

type ProvisionException struct {
	RequestID     string    // Match up this with the request
	Reference     string    // A reference to the thing that was provisioned - device code or sub_product_code
	ReferenceType string    // The name of the field that the reference pertains to
	Time          time.Time // Time that provisioning completed
	System        string    // The provisioning sub-system that had the issue
	Tag           string    // A simple tag to define what sort of problem this is
	Alert         bool      // Get someones attention immediately
	Error         string    // Human readable text about what the outcome was
}

/*
{"RequestID": "08923546y","AccountCode": "ACCT0160","AccountName": "telMAX","SubscribeCode": "SUBS001","SiteID": "093g56","SubscribeName": "Tim St. Pierre","RequestType": "New","RequestUser": "tstpierre"}



*/
