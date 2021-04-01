package main

import (
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

func main() {
	http.HandleFunc("/", TestHandler)

	http.ListenAndServe(":8080", nil)

}

func TestHandler(w http.ResponseWriter, r *http.Request) {
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Got an error %v", err)
	}
	log.Infof("Path is:\n %v \n Headers are:\n%v\nBody is:\n %v", r.URL.Path[1:], r.Header, string(reqBody))
}
