package main

import (
	//	"bitbucket.org/timstpierre/telmax-provision/dhcpdb"
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"ipint"
)

var (
	SQL     *sql.DB
	V4Net   = flag.String("v4net", "", "IPv4 subnet")
	V4CIDR  = flag.Int("v4cidr", 22, "IPv4 CIDR Length")
	V6Net   = flag.String("v6net", "", "IPv6 subnet")
	V6PDLen = flag.Int("v6len", 60, "IPv6 delegated prefix length")
	Node    = flag.String("node", "Brooklin", "Routing node")
	Pool    = flag.String("pool", "Residential", "DHCP Pool Name")
	Vlan    = flag.Int("vlan", 0, "Vlan ID for the block")
	Subnet  = flag.Int("subnet", 0, "Subnet ID")
)

func SQLConnect() *sql.DB {
	s := sqlServer{
		Hostname: "dhcp01.lab.dc1.osh.telmax.ca",
		Username: "provisioning",
		Password: "telMAXProv720",
	}
	d := "dhcp"
	connect_str := s.Username + ":" + s.Password + "@tcp(" + s.Hostname + ":3306)/" + d
	db, err := sql.Open("mysql", connect_str)
	if err != nil {
		log.Fatal(err)
		panic(err.Error())
	}
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
		panic(err.Error())
	}
	log.Info("Connected to SQL server - using database " + d)
	return db
}

func init() {
	flag.Parse()
	lvl, _ := log.ParseLevel(*LogLevel)
	log.SetLevel(lvl)
	SQL = SQLConnect
}

func main() {

}
