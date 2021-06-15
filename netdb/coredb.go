// Implement a provisioning API to query status and request provisioning of network elements on the telMAX network.
//
package netdb

import (
	"context"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	Database mongo.Database
)

// The structure that corresponds with the access_ports collection in coreDB.  This represents each access circuit to a subscriber endpoint.
//
type accessPort struct {
	CircuitID        string     `bson:"circuit_id"`
	CircuitCode      string     `bson:"circuit_code"`
	AccessTechnology string     `bson:"access_technology"`
	NetworkElement   string     `bson:"network_element"`
	CustomerCode     string     `bson:"customer_code"`
	SubscribeCode    string     `bson:"subscribe_code"`
	Status           string     `bson:"access_port_status"`
	AdminState       string     `bson:"adminState"`
	AccessData       accessData `bson:"accessData"`
	TestData         []testData `bson:"TestData,omitempty"`
}

// A sub-object of an accessPort, this is the technological description of the physical port itself.
//
type accessData struct {
	Platform   string `bson:"access_platform"`
	OltSlot    int    `bson:"olt_slot,omitempty"`
	OltPort    int    `bson:"olt_port,omitempty"`
	OltONU     int    `bson:"olt_onu,omitempty"`
	SwitchID   string `bson:"switch_id,omitempty"`
	SwitchPort string `bson:"switch_port,omitempty"`
	Optic      string `bson:"optic,omitempty"`
	Wavelength string `bson:"wavelength,omitempty"`
}

// A structure to record optical test results in the history of the circuit
//
type testData struct {
	Date            string
	Method          string
	UpstreamPower   float32
	DownstreamPower float32
	Description     string
}

// Details about a specific service product from tmbill
//
type productData struct {
	ProductCode    string          `bson:"product_code"`
	Category       string          `bson:"category"`
	ProductName    string          `bson:"product_name"`
	NetworkProfile *networkProfile `bson:"network_profile"`
	ProductList    []string        `bson:"member_of_package"`
}

// An optional profile attached to a productData structure that defines what network resources are requered, and how to provision them.
type networkProfile struct {
	Name           string `bson:"profile_name"`
	Vlan           int    `bson:"vlan,omitempty"`
	AddressPool    string `bson:"address_pool,omitempty"`
	RoutingMode    string `bson:"routing_mode"`
	Igmp           bool   `bson:"igmp_enable"`
	InputFilter    string `bson:"input_filter,omitempty"`
	OutputFilter   string `bson:"output_filter,omitempty"`
	GemOffset      int    `bson:"gem_offset,omitempty"`
	TrafficProfile string `bson:"traffic_profile,omitempty"`
}

// Open a connection to the Core MongoDB and return a database object.  The assumption is that only one database is required.
//
func CoredbConnect() *mongo.Database {
	clientOptions := options.Client().ApplyURI("mongodb://52.60.223.89:27017")
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	log.Info("Connected to MongoDB!")

	database := client.Database("maxbill")

	return database
}

// Get access port data object by looking up circuit_id
//
func GetaccessPortbyID(id string) accessPort {
	var result accessPort
	filter := bson.D{{
		"circuit_id",
		bson.D{{
			"$in",
			bson.A{
				id,
			},
		}},
	}}
	err := Database.Collection("access_ports").FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		log.Info(err)
	}
	return result
}

// Update the access port status
//
func UpdateaccessPortStatus(id string, status string) bool {

	filter := bson.D{{
		"circuit_id",
		id,
	}}

	update := bson.D{
		{"$set", bson.D{
			{"access_port_status", status},
		}},
	}
	updateResult, err := Database.Collection("access_ports").UpdateOne(context.TODO(), filter, update)
	if err != nil {
		log.Info(err)
	}

	if updateResult.MatchedCount == 1 {
		log.Info("update status for circuit " + id + " to " + status)
		return true
	} else {
		return false
	}

}

// Get all access products assigned to a combination of customer_code and subscribe_code
//
func GetaccessProduct(cust string, subs string) []productData {
	var results []*productData
	var packages []*productData

	filter := bson.D{
		{
			"subscribe_code",
			bson.D{
				{
					"$in",
					bson.A{
						subs,
					},
				},
			},
		},
		{"customer_code",
			bson.D{
				{
					"$in",
					bson.A{
						cust,
					},
				},
			},
		},
	}

	cur, err := Database.Collection("subscribe_products").Find(context.TODO(), filter)
	if err != nil {
		log.Fatal(err)
	}
	for cur.Next(context.TODO()) {

		// create a value into which the single document can be decoded
		var elem productData
		err := cur.Decode(&elem)
		if err != nil {
			log.Fatal(err)
		}
		if elem.Category == "Package" {
			packages = append(packages, &elem)
			log.Info("Adding Package " + elem.ProductName + " to list")
		} else {
			log.Info("Adding Product " + elem.ProductName + " to list")
			results = append(results, &elem)
		}
	}
	if err := cur.Err(); err != nil {
		log.Fatal(err)
	}

	// Close the cursor once finished
	cur.Close(context.TODO())

	var product_list []productData
	var product_data productData

	// Iterate through all the packages and get the product codes that belong to them
	//

	for _, packageItem := range packages {
		log.Info("Looking up contents of package " + packageItem.ProductName + " code " + packageItem.ProductCode)

		pkgfilter := bson.D{{
			"member_of_package",
			packageItem.ProductCode,
		}}

		cur, err := Database.Collection("products").Find(context.TODO(), pkgfilter)
		if err != nil {
			log.Fatal(err)
		}

		for cur.Next(context.TODO()) {

			var elem productData
			err := cur.Decode(&elem)
			if err != nil {
				log.Fatal(err)
			}
			if elem.Category == "Package" {
				packages = append(packages, &elem)
				log.Info("Nested package! Package " + elem.ProductName + " to list")
			} else {
				log.Info("Adding package member " + elem.ProductName)
				results = append(results, &elem)
			}

		}

		/*
			for _, packageEntry := range packageItem.ProductList {
				product_data = GetProductData(packageEntry)
				log.Info("Adding member of package " + packageEntry + " " + product_data.ProductName)
				product_list = append(product_list, product_data)

			}
		*/
		// Close the cursor once finished

	}
	cur.Close(context.TODO())

	// Iterate through all these products and get their product data.  Add them to the list
	//
	for _, sub_product := range results {
		product_data = GetProductData(sub_product.ProductCode)
		product_list = append(product_list, product_data)
	}

	return product_list

}

func GetProductData(code string) productData {
	log.Info("Getting product data for " + code)
	var product_data productData
	filter := bson.D{{
		"product_code",
		bson.D{{
			"$in",
			bson.A{
				code,
			},
		}},
	}}
	err := Database.Collection("products").FindOne(context.TODO(), filter).Decode(&product_data)
	if err != nil {
		log.Info(err)
	}
	log.Debug(product_data.ProductName + " " + product_data.Category)
	return product_data
}
