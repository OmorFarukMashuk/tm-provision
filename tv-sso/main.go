package main

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
	LogLevel = flag.String("loglevel", "warn", "Log Level")
	Listen   = flag.String("listen", ":5009", "HTTP API listen address:port")

	UseTLS  = flag.Bool("tls.enable", false, "Enable TLS")
	TLSCert = flag.String("tls.cert", "/etc/ssl/tvauth.crt", "LDAP Server Certificate")
	TLSKey  = flag.String("tls.key", "/etc/ssl/private/tvauth.key", "LDAP Server private key")

	MongoURI     = flag.String("mongouri", "mongodb://coredb01.dc1.osh.telmax.ca:27017", "MongoDB URL for telephone database")
	CoreDatabase = flag.String("coredatabase", "telmaxmb", "Core Database name")

	DBClient *mongo.Client
	CoreDB   *mongo.Database
)

func init() {
	flag.Parse()
	lvl, _ := log.ParseLevel(*LogLevel)
	log.SetLevel(lvl)

	DBClient = telmax.DBConnect(*MongoURI, "maxcoredb", "coredbmax955TEL")
	if DBClient != nil {
		CoreDB = DBClient.Database(*CoreDatabase)
	}
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

				} else {

				}

				//else if s == syscall.SIGINFO {
				//	log.Warningf("ticketapi listening on %s TLS %t current debug level %s", *Listen, *UseTLS, *LogLevel)
				//			return
				//}

			}
		}
	}()
	router := mux.NewRouter().StrictSlash(false)
	router.HandleFunc("/", HandleCheckAuth).Methods("GET")

	srv := http.Server{
		Addr:         *Listen,
		WriteTimeout: 5 * time.Second,
		Handler:      router,
	}
	// Start the HTTP or HTTPS server
	if *UseTLS {
		log.Warning("Listening on " + *Listen + " TLS")
		log.Fatal(srv.ListenAndServeTLS(*TLSCert, *TLSKey))
	} else {
		log.Warning("Listening on " + *Listen)
		log.Fatal(srv.ListenAndServe())
	}
}

// Shut down the app cleanly
func AppCleanup() {
	log.Error("Stopping TV Auth Service")
	DBClient.Disconnect(context.TODO())
}
