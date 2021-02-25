package provisionproducer

import (
	"bitbucket.org/timstpierre/telmax-provision/structs"
	//"flag"
	//"github.com/Shopify/sarama"
	//cluster "github.com/bsm/sarama-cluster"
	//log "github.com/sirupsen/logrus"
	//"go.mongodb.org/mongo-driver/bson"
)

type State struct {
}

var (
	ProvisionProducer *State
	ProvisionTopic    = "provisionrequest"
)

func init() {
	// Set up the connection to Kafka

}

func (state *State) SubmitRequest(request telmaxprovision.ProvisionRequest) {
	// Submit the request to the topic

}
