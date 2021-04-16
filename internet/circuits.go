package main

import ()

var ()

func AssignCircuit(wirecentre string, pon string, subscriber string) (circuit Circuit, err error) {

}

type Circuit struct {
	ID         string `bson:"circuit_id"`    // A unique identifier for this circuit
	Wirecenter string `bson:wirecentre`      // The wirecentre this circuit originates at
	AccessNode string `bson:"access_node"`   // The name of the device this circuit is attached to
	Interface  string `bson:"interface"`     // The interface on the access node
	Unit       int    `bson:"onu,omitempty"` // The ONU or unit number
	Subscriber string `bson:"subscriber"`    // The subscriber ID ACCT-SUBS

}
