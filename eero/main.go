package main

/*

The EERO service handles the REST API provisioning method of the Eero Wi-Fi Mesh solution.
The EERO is an "RG" category device, but differs from the "smartRG" in that it does not use an ACS.

*/

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"

	"bitbucket.org/timstpierre/telmax-provision/kafka"
	telmaxprovision "bitbucket.org/timstpierre/telmax-provision/structs"

	"bitbucket.org/telmaxnate/eero"
)

var (
	logLevel   = flag.Int("log", 4, "Set logging filter: [0:Panic], [1:Fatal], [2:Error], [3:Warn], [4:Info], [5:Debug], [6:Trace]")
	TZLocation *time.Location
	KafkaTopic = flag.String("kafka.topic", "provisionrequest", "Kafka topic to consume from")
	KafkaBrk   = flag.String("kafka.brokers", "kfk01.tor2.telmax.ca:9092, kfk02.tor2.telmax.ca:9092, kfk03.tor2.telmax.ca:9092", "Kafka brokers list separated by commas") // Temporary default
	KafkaGroup = flag.String("kafka.group", "eero", "Kafka group id")                                                                                                      // Change this to your provision subsystem name

	MongoURI       = flag.String("mongo.uri", "mongodb://coredb.telmax.ca:27017", "MongoDB URL for telmax database")
	MongoUser      = flag.String("mongo.user", "maxcoredb", "MongoDB User")
	MongoPass      = flag.String("mongo.pass", "coredbmax955TEL", "MongoDB Password")
	CoreDatabase   = flag.String("mongo.core", "telmaxmb", "Core Database name")
	TicketDatabase = flag.String("mongo.ticket", "maxticket", "Ticketing Database name")

	//DBClient *mongo.Client
	CoreDB   *mongo.Database
	TicketDB *mongo.Database

	userEmail = flag.String("eero.email", "eero@telmax.com", "Eero User Email")
	accessKey = flag.String("eero.key", "15974148|12d467oiahrvacdvfv1f4jl2gs", "Eero API Access Key returned from Login Post")
	tempCode  = flag.Int("eero.code", 0, "Eero API Verification Code sent to Email")
	eeroApi   = &eero.EeroApi{Gateway: "api-user.e2ro.com"}
	brokers   []string
	topics    []string

	SQLHost = flag.String("dhcpdb.host", "dhcp04.tor2.telmax.ca", "DHCP SQL hostname")
	SQLUser = flag.String("dhcpdb.user", "provisioning", "DHCP SQL Username")
	SQLPass = flag.String("dhcpdb.pass", "telMAXProv720", "DHCP SQL Password")
	DhcpDB  *sql.DB
)

//	The state object is mostly used to maintain the state for the Kafka consumer and the database handle

func init() {
	flag.Parse()
	eeroApi.Key = *accessKey
	if eeroApi.Key == "" {
		err := eeroApi.Login(*userEmail)
		if err != nil {
			log.Fatalln(err)
		} else {
			log.Fatalf("Key retrieved: %s\nYou should be receiving an email with a verification code.\n", eeroApi.Key)
		}
	}
	// Overwrite default value with code and supply key to verify.
	// Only used to verify email first time.
	eeroApi.Code = *tempCode
	if eeroApi.Code != 0 {
		err := eeroApi.Verify()
		if err != nil {
			log.Fatalln(err)
		} else {
			log.Infoln("User verified, proceed")
		}
	}
	lvl := uint32(*logLevel)
	if lvl > 6 {
		lvl = 6
	}
	log.SetLevel(log.Level(lvl))
	TZLocation, _ = time.LoadLocation("America/Toronto")
	CoreDB = eeroApi.InitDB(*MongoURI, *MongoUser, *MongoPass, *CoreDatabase, "eero")
	DhcpDB = SQLConnect()

	brokers = strings.Split(*KafkaBrk, ",")
	if len(brokers) < 1 {
		log.Fatalf("no Kafka brokers!")
	}
	topics = strings.Split(*KafkaTopic, ",")
	if len(topics) < 1 {
		log.Fatalf("no Kafka topics!")
	}
	kafka.StartProducer(brokers)
}

func main() {
	// setup signal catching
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs)
	// method invoked upon seeing signal
	go func() {
		for {
			select {
			case signal := <-sigs:
				if signal == syscall.SIGQUIT || signal == syscall.SIGKILL || signal == syscall.SIGTERM || signal == syscall.SIGINT {
					log.Infof("RECEIVED SIGNAL: %s", signal)
					AppCleanup()
					os.Exit(1)
				} else if signal == syscall.SIGHUP {
					log.Warning("Re-running process routine")

				}
				// This only works on FreeBSD and MacOS, but it can be nice
				//else if s == syscall.SIGINFO {
				//			return
				//}
			}
		}
	}()
	go auditDaemon()
	kafka.StartConsumer(brokers, topics, *KafkaGroup, MessageHandler)

}

// Quit cleanly - close any database connections or other open sockets here.
func AppCleanup() {
	log.Error("Stopping Application")
	kafka.StopConsumer()
	kafka.Shutdown()
	CoreDB.Client().Disconnect(context.TODO())
	DhcpDB.Close()
}

func MessageHandler(topic string, timestamp time.Time, data []byte) {
	log.Infof("Kafka message %v, %v", topic, string(data))
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

func SQLConnect() *sql.DB {
	connect_str := *SQLUser + ":" + *SQLPass + "@tcp(" + *SQLHost + ":3306)/dhcp"
	db, err := sql.Open("mysql", connect_str)
	if err != nil {
		log.Errorf("Connecting to DHCPDB - %v", err)
	}
	err = db.Ping()
	if err != nil {
		log.Errorf("Checking Connection to DHCPDB - %v", err)
	}
	log.Infof("Connected to DHCPDB")
	return db
}
