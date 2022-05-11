package netdb

import (
	"context"
	"errors"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ()

func AllocateCircuit(db *mongo.Database, wirecentre string, pon string, subscriber string) (circuit Circuit, assigned bool, err error) {
	existingCircuit, err := GetSubscriberCircuit(db, subscriber)
	if existingCircuit.ID != "" {
		assigned = false
		circuit = existingCircuit
		log.Infof("Subscriber %v already has circuit %v", subscriber)
		return
	} else {
		err = nil
	}
	circuit, err = GetNextCircuit(db, pon)
	if err != nil {
		log.Errorf("Problem getting available circuit - %v", err)
		return
	}
	err = AssignCircuit(db, circuit.ID, subscriber)
	if err != nil {
		log.Errorf("Problem assigning circuit - %v", err)
		return
	}
	assigned = true
	return
}

func GetSubscriberCircuit(db *mongo.Database, subscriber string) (circuit Circuit, err error) {
	filter := bson.D{{"subscriber", subscriber}}
	err = db.Collection("access_ports").FindOne(context.TODO(), filter).Decode(&circuit)
	if err != nil {
		log.Debugf("Problem getting circuit for subscriber %v %v", subscriber, err)
	}
	return
}

func GetCircuit(db *mongo.Database, id string) (circuit Circuit, err error) {
	filter := bson.D{{"circuit_id", id}}
	err = db.Collection("access_ports").FindOne(context.TODO(), filter).Decode(&circuit)
	if err != nil {
		log.Infof("Problem getting circuit %v", err)
	}
	return
}

func AssignCircuit(db *mongo.Database, id string, subscriber string) error {
	update := bson.D{{
		"$set", bson.D{
			{"status", "Assigned"},
			{"subscriber", subscriber},
		},
	}}
	result, err := db.Collection("access_ports").UpdateOne(context.TODO(), bson.D{{"circuit_id", id}}, update)
	if result.MatchedCount == 1 {
		log.Infof("Circuit %v assigned successfully", id)
	} else {
		err = errors.New("Circuit not successfully assigned - no change made")
	}
	return err
}

func ReleaseCircuit(db *mongo.Database, id string) error {
	update := bson.D{{
		"$set", bson.D{
			{"status", "Available"},
			{"subscriber", ""},
		},
	}}
	result, err := db.Collection("access_ports").UpdateOne(context.TODO(), bson.D{{"circuit_id", id}}, update)
	if result.MatchedCount == 1 {
		log.Info("Circuit %v released successfully", id)
	} else {
		err = errors.New("Circuit not successfully released - no change made")
	}
	return err
}

func GetNextCircuit(db *mongo.Database, iface string) (circuit Circuit, err error) {
	log.Infof("Finding available circuit for interface %v", iface)
	findOptions := options.Find()
	findOptions.SetLimit(1)
	findOptions.SetCollation(&options.Collation{Locale: "en_US", NumericOrdering: true})
	findOptions.SetSort(bson.D{{"circuit_id", 1}})
	filter := bson.D{{
		"$and", bson.A{
			bson.M{"status": "Available"},
			bson.M{"interface": iface},
		},
	}}
	cur, err := db.Collection("access_ports").Find(context.TODO(), filter, findOptions)

	if err != nil {
		log.Errorf("Problem looking up available circuits %v", err)
	}

	if cur.Next(context.TODO()) {
		err = cur.Decode(&circuit)
		return

	} else {
		err = errors.New("No circuits available for interface " + iface)
	}
	return
}

type Circuit struct {
	ID          string `bson:"circuit_id"`    // A unique identifier for this circuit
	Wirecentre  string `bson:wirecentre`      // The wirecentre this circuit originates at
	RoutingNode string `bson:"routing_node"`  // THe Layer 3 device serving this circuit
	AccessNode  string `bson:"access_node"`   // The name of the device this circuit is attached to
	Interface   string `bson:"interface"`     // The interface on the access node
	Unit        int    `bson:"onu,omitempty"` // The ONU or unit number
	Subscriber  string `bson:"subscriber"`    // The subscriber ID ACCT-SUBS
	Status      string `bson:"status"`        // Available, Assigned, Reserved

}
