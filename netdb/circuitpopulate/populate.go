/*
	Simple utility for populated network database with PON circuits

*/
package main

import (
	"bitbucket.org/telmaxdc/telmax-provision/netdb"
	"context"
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	LogLevel    = flag.String("loglevel", "info", "Log Level")
	Technology  = flag.String("technology", "xgspon", "Access Technology type - gpon, 1gigethernet, xgspon, etc")
	Wirecentre  = flag.String("wirecentre", "Stouffville", "The wirecentre this circuit belongs to")
	AccessNode  = flag.String("accessnode", "stouffville-olt11", "The hostname or element name of the access device")
	RoutingNode = flag.String("routingnode", "stouffville2", "The routing node name used for dhcp assignment")
	Ports       = flag.Int("ports", 16, "The number of physical ports on the unit")
	Split       = flag.Int("split", 64, "The split ratio for PON systems - number of ONUs por OLT port")
	MongoURI    = flag.String("mongouir", "mongodb://coredb.telmax.ca:27017", "The URI of the database to connect to")
	Database    *mongo.Database
)

func init() {
	flag.Parse()
	lvl, _ := log.ParseLevel(*LogLevel)
	log.SetLevel(lvl)

	clientOptions := options.Client().ApplyURI(*MongoURI)
	clientOptions.SetAuth(options.Credential{
		Username: "maxcoredb",
		Password: "coredbmax955TEL",
	})
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatalf("Problem connecting to Mongo URI %v, received errer %v", *MongoURI, err)
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatalf("Problem pinging server %v, received error %v", *MongoURI, err)
	} else {
		log.Infof("Connected to MongoDB %v", *MongoURI)
	}
	Database = client.Database("network")

}

func main() {
	port := 1
	onu := 1
	//	index := 0
	ccint := LastCode(*Database.Collection("access_ports"))

	var accessPorts []interface{}

	switch *Technology {
	case "gpon", "xgspon":
		//		var portText string
		//		var onuText string

		var circuit_code_text string

		for port <= *Ports {

			//			portText = strconv.Itoa(port)
			for onu <= *Split {
				//				onuText = strconv.Itoa(onu)
				circuit_code_text = fmt.Sprintf(strings.ToLower(*Wirecentre)+"-%05d", ccint)

				circuitData := netdb.Circuit{
					ID:          circuit_code_text,
					Wirecentre:  *Wirecentre,
					RoutingNode: *RoutingNode,
					AccessNode:  *AccessNode,
					Interface:   fmt.Sprintf(*AccessNode+"-pon%02d", port),
					Unit:        onu,
					Status:      "Available",
				}
				onu++
				ccint++
				accessPorts = append(accessPorts, circuitData)
				log.Infof("New circuit %v", circuitData)
			}
			port++
			onu = 1
		}
	}
	log.Printf("New circuit data would be %v", accessPorts)

	_, err := Database.Collection("access_ports").InsertMany(context.TODO(), accessPorts)
	if err != nil {
		log.Fatal(err)
	}
	//log.Infof("Insert result is %v", insertResult)

}

func LastCode(collection mongo.Collection) int {
	// Get last circuit code
	var lastCircuit netdb.Circuit

	findOptions := options.Find()
	findOptions.SetLimit(1)
	findOptions.SetCollation(&options.Collation{Locale: "en_US", NumericOrdering: true})
	findOptions.SetSort(bson.D{{"circuit_id", -1}})

	cur, err := collection.Find(context.TODO(), bson.D{{"wirecentre", *Wirecentre}}, findOptions)

	if err != nil {
		log.Fatal(err)
	}
	var ccint int
	defer cur.Close(context.TODO())
	if cur.Next(context.TODO()) {
		err = cur.Decode(&lastCircuit)
		log.Infof("Last circuit is %v", lastCircuit)
		codeArray := strings.Split(lastCircuit.ID, "-")
		ccint, _ = strconv.Atoi(codeArray[1])
		fmt.Sprintf("Last record is %d", ccint)
		ccint += 1

	} else {
		ccint = 1
		fmt.Println("no records - starting at 1")
	}
	return ccint

}
