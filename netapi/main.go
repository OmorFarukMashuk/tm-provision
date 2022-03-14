package main

/*
	NETAPI back-end provides UI interaction to network infrastructure for troubleshooting and information operations.
*/

import (
	"bitbucket.org/telmaxdc/telmax-common"
	"context"
	"flag"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var (
	LogLevel = flag.String("loglevel", "info", "Log Level")
	Listen   = flag.String("listen", ":5011", "HTTP API listen address:port")

	UseTLS  = flag.Bool("tls.enable", false, "Enable TLS")
	TLSCert = flag.String("tls.cert", "/etc/ssl/netapi.crt", "LDAP Server Certificate")
	TLSKey  = flag.String("tls.key", "/etc/ssl/private/netapi.key", "LDAP Server private key")

	APIKey = flag.String("apikey", "098g5467o456n78s8e5878c8ty4578uihdsrc", "API Key used for simple authentication")

	MongoURI     = flag.String("mongouri", "mongodb://coredb01.dc1.osh.telmax.ca:27017", "MongoDB URL for telephone database")
	NetDatabase  = flag.String("netdatabase", "network", "Network database")
	CoreDatabase = flag.String("coredatabase", "telmaxmb", "Core Database name")

	TicketDatabase = flag.String("ticketdatabase", "maxticket", "Database for ticketing")
	CRMDatabase    = flag.String("crmdatabase", "maxcrm", "Database for CRM")

	TZLocation *time.Location
	DBClient   *mongo.Client
	CoreDB     *mongo.Database
	TicketDB   *mongo.Database
	NetDB      *mongo.Database
)

func init() {
	flag.Parse()
	lvl, _ := log.ParseLevel(*LogLevel)
	log.SetLevel(lvl)

	DBClient = telmax.DBConnect(*MongoURI, "maxcoredb", "coredbmax955TEL")
	if DBClient != nil {
		NetDB = DBClient.Database(*NetDatabase)
		CoreDB = DBClient.Database(*CoreDatabase)
		TicketDB = DBClient.Database(*TicketDatabase)
	}
	TZLocation, _ = time.LoadLocation("America/Toronto")
}

func main() {

	// setup signal catching
	sigs := make(chan os.Signal, 1)
	// catch all signals since not explicitly listing
	signal.Notify(sigs)
	//signal.Notify(sigs,syscall.SIGQUIT)
	// method invoked upon seeing signal
	go func() {
		for {
			select {
			case s := <-sigs:
				log.Debugf("RECEIVED SIGNAL: %s", s)
				if s == syscall.SIGQUIT || s == syscall.SIGKILL || s == syscall.SIGTERM || s == syscall.SIGINT {
					AppCleanup()
					os.Exit(1)
				} else if s == syscall.SIGHUP {
					log.Warning("Re-running process routine")

				} else {

				}

				//else if s == syscall.SIGINFO {
				//	log.Warningf("ticketapi listening on %s TLS %t current debug level %s", *Listen, *UseTLS, *LogLevel)
				//			return
				//}

			}
		}
	}()

	// Run the web server
	router := mux.NewRouter().StrictSlash(false)
	router.Methods("OPTIONS").HandlerFunc(HandleOptions)

	// Here's the API routes
	router.HandleFunc("/dummy/{accountcode}/{subscribecode}", HandleDummyTest).Methods("GET")
	router.HandleFunc("/onustatus/{accountcode}/{subscribecode}", HandleONUStatus).Methods("GET")

	if *UseTLS {
		log.Warning("Listening on " + *Listen + " TLS")
		log.Fatal(http.ListenAndServeTLS(*Listen, *TLSCert, *TLSKey, router))
	} else {
		log.Warning("Listening on " + *Listen)
		log.Fatal(http.ListenAndServe(*Listen, router))
	}
}

// Shut down the app cleanly
func AppCleanup() {
	log.Error("Stopping Net API Service")
	DBClient.Disconnect(context.TODO())
}

// Handle Options pre-flight requests
func HandleOptions(w http.ResponseWriter, r *http.Request) {
	CORSHeaders(w, r)
}

// Generate CORS headers for responses
func CORSHeaders(w http.ResponseWriter, r *http.Request) {
	headers := w.Header()
	//	log.Infof("Request Origin is: %v", r.Header.Get("Origin"))
	headers.Add("Access-Control-Allow-Headers", "Content-Type, Origin, Accept, token, api-key")
	headers.Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	//	headers.Add("Access-Control-Max-Age", "3600")
	headers.Add("Access-Control-Allow-Origin", "*")

}

// Check API Key Authorization
func CheckAuth(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("api-key") == *APIKey {
		return true
	}
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false
}

// Consistent respons structure - error only exists if there is an error.  Status is always "ok" or "error"
type Response struct {
	Status string      `json:"status"`
	Error  string      `json:"error,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}
