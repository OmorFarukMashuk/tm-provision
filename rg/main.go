package main

/*

Scaffolding code to make your service.  Replace this with a description of what your service does

*/

import (
	//"bson"
	"encoding/json"
	"flag"
	//	"github.com/Shopify/sarama"
	"bitbucket.org/telmaxdc/telmax-common"
	"context"
	log "github.com/sirupsen/logrus"
	//	"go.mongodb.org/mongo-driver/bson"
	"bitbucket.org/telmaxdc/telmax-provision/kafka"
	"bitbucket.org/telmaxdc/telmax-provision/structs"
	"go.mongodb.org/mongo-driver/mongo"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var (
	LogLevel   = flag.String("loglevel", "info", "Log Level")
	TZLocation *time.Location
	KafkaTopic = flag.String("kafka.topic", "provisionrequest", "Kafka topic to consume from")
	KafkaBrk   = flag.String("kafka.brokers", "kf01.dc1.osh.telmax.ca:9092", "Kafka brokers list separated by commas") // Temporary default
	KafkaGroup = flag.String("kafka.group", "rg", "Kafka group id")                                                    // Change this to your provision subsystem name

	MongoURI       = flag.String("mongouri", "mongodb://coredb01.dc1.osh.telmax.ca:27017", "MongoDB URL for telephone database")
	CoreDatabase   = flag.String("coredatabase", "telmaxmb", "Core Database name")
	TicketDatabase = flag.String("ticketdatabase", "maxticket", "Database for ticketing")

	DBClient *mongo.Client
	CoreDB   *mongo.Database
	TicketDB *mongo.Database
)

//	The state object is mostly used to maintain the state for the Kafka consumer and the database handle

func init() {
	flag.Parse()
	lvl, _ := log.ParseLevel(*LogLevel)
	log.SetLevel(lvl)
	TZLocation, _ = time.LoadLocation("America/Toronto")

	// Connect to the database
	DBClient = telmax.DBConnect(*MongoURI, "maxcoredb", "coredbmax955TEL")
	if DBClient != nil {
		CoreDB = DBClient.Database(*CoreDatabase)
		TicketDB = DBClient.Database(*TicketDatabase)
	}

	brokers := strings.Split(*KafkaBrk, ",")
	kafka.StartProducer(brokers)

}

func main() {
	// setup signal catching
	sigs := make(chan os.Signal, 1)

	brokers := strings.Split(*KafkaBrk, ",")
	topics := strings.Split(*KafkaTopic, ",")

	// catch all signals since not explicitly listing
	signal.Notify(sigs)
	//signal.Notify(sigs,syscall.SIGQUIT)
	// method invoked upon seeing signal
	go func() {
		for {
			select {

			case signal := <-sigs:
				log.Debugf("RECEIVED SIGNAL: %s", signal)
				if signal == syscall.SIGQUIT || signal == syscall.SIGKILL || signal == syscall.SIGTERM || signal == syscall.SIGINT {
					AppCleanup()
					os.Exit(1)
				} else if signal == syscall.SIGHUP {
					log.Warning("Re-running process routine")

				} else {

				}
				// This only works on FreeBSD and MacOS, but it can be nice
				//else if s == syscall.SIGINFO {
				//			return
				//}

			}
		}
	}()

	kafka.StartConsumer(brokers, topics, *KafkaGroup, MessageHandler)

}

// Quit cleanly - close any database connections or other open sockets here.
func AppCleanup() {
	log.Error("Stopping Application")
	kafka.StopConsumer()
	kafka.Shutdown()
	DBClient.Disconnect(context.TODO())

}

func MessageHandler(topic string, timestamp time.Time, data []byte) {
	log.Infof("Kafka message %v, %v, %v", topic, timestamp, string(data))
	switch topic {
	case "provisionrequest":
		// Create an empty request object
		var request telmaxprovision.ProvisionRequest

		//	Unmarshal the bson serialized message into an object
		err := json.Unmarshal(data, &request)
		if err != nil {
			log.Warnf("unmarshaling error: %v", err)
		} else {
			log.Debug(request)
			HandleProvision(request)
		}
	}
}
