package kafka

import (
	"bitbucket.org/timstpierre/telmax-provision/structs"
	//"flag"
	"encoding/json"
	"github.com/Shopify/sarama"
	"github.com/google/uuid"
	//cluster "github.com/bsm/sarama-cluster"
	log "github.com/sirupsen/logrus"
	//"go.mongodb.org/mongo-driver/bson"
)

var (
	ProvisionProducer sarama.SyncProducer
	ProvisionTopic    = "provisionrequest"
	KafkaBrokers      = []string{"kf01.dc1.osh.telmax.ca:9092", "kf02.dc1.osh.telmax.ca:9092", "kf03.dc1.osh.telmax.ca:9092"}
)

/*
func init() {
	// Set up the connection to Kafka
	ProvisionProducer = NewProducer(KafkaBrokers)
	sarama.Logger = log.New()

}
*/
func StartProducer(brokers []string) {
	ProvisionProducer = NewProducer(brokers)
	if ProvisionProducer != nil {
		log.Info("Connected to Kafka cluster!")
	}
}

func SubmitRequest(request telmaxprovision.ProvisionRequest) (id string, err error) {
	request.RequestID = uuid.New().String()
	data, err := json.Marshal(request)
	//	request.Create()
	// Submit the request to the topic
	message := sarama.ProducerMessage{
		Topic: ProvisionTopic,
		Value: sarama.ByteEncoder(data),
	}
	partition, offset, err := ProvisionProducer.SendMessage(&message)
	log.Infof("Partition is %v and offset is %v", partition, offset)
	if err != nil {
		log.Errorf("Kafka producer error %v", err)
	}
	//id = partition + offset
	id = request.RequestID
	return
}

func NewProducer(brokers []string) sarama.SyncProducer {
	config := sarama.NewConfig()
	log.Info("brokers list is %v", brokers)
	config.Producer.Retry.Max = 10 // Retry up to 10 times to produce the message
	config.Producer.Return.Successes = true
	/*
		tlsConfig := createTlsConfiguration()
		if tlsConfig != nil {
			config.Net.TLS.Config = tlsConfig
			config.Net.TLS.Enable = true
		}
	*/
	// On the broker side, you may want to change the following settings to get
	// stronger consistency guarantees:
	// - For your broker, set `unclean.leader.election.enable` to false
	// - For the topic, you could increase `min.insync.replicas`.

	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Fatalf("Failed to start Sarama producer:", err)
	}

	return producer
}

func Shutdown() {
	ProvisionProducer.Close()
}
