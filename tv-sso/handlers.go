package main

import (
	"bitbucket.org/telmaxdc/telmax-common/maxbill"
	"encoding/xml"
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
)

var ()

func HandleCheckAuth(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if ok && username != "" {
		log.Infof("Username is %v password is %v", username, password)
		subscribes, err := maxbill.GetSubscribes(CoreDB, "tv_username", username)
		if err != nil {
			log.Errorf("Problem getting TV account with username %v, %v", username, err)
			http.Error(w, "Authentication Failed", http.StatusUnauthorized)
			return
		}
		log.Infof("Account data is %v", subscribes)
		if len(subscribes) > 0 {
			accountdata := subscribes[0]
			if accountdata.TVPassword == password {
				var account AccountResponse
				account.Account = accountdata.AccountCode + accountdata.SubscribeCode
				fmt.Fprintf(w, xml.Header)
				xml.NewEncoder(w).Encode(account)
				return
			}
		}
	}
	http.Error(w, "Authentication Failed", http.StatusUnauthorized)

}

type AccountResponse struct {
	XMLName xml.Name `xml:"account"`
	Account string   `xml:",chardata"`
}
