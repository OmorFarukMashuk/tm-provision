package dhcpdb

import (
	"context"
	"database/sql"
	//"fmt"
	"github.com/davecgh/go-spew/spew"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"ipint"
	"net"
	"strconv"
)

type sqlServer struct {
	Hostname string
	Username string
	Password string
}

type ipReservation struct {
	HostID int
	DhcpID string
	Pool   string
	VlanID int
	V4Addr net.IP
	V6wan  net.IP
	V6dp   net.IP
	V6size int
}

type ipv6Reservation struct {
	ResID   int
	Address net.IP
	Length  int
	Type    int
	Iaid    int
	HostID  int
}

var (
	SQL *sql.DB
)

func SQLConnect() *sql.DB {
	s := sqlServer{
		Hostname: "dhcp01.dc1.osh.telmax.ca",
		Username: "provisioning",
		Password: "Pr0vision!",
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

// Assigns an address from the named pool and returns the address object and the success of the operation.  Will return existing allocation if already assigned.
//
func DhcpAssign(ctx context.Context, pool string, cust string, subs string, circuit string, dhcpid string) (bool, ipReservation) {
	db := SQLConnect()
	defer db.Close()

	log.Info("Getting address reservation for circuit " + circuit)
	success, res := dhcpGetAssign(ctx, circuit)

	if success {
		log.Info("Existing address already allocated")
		return success, res
	} else {
		log.Info("Allocating new address")

		result, err := db.ExecContext(ctx, `update hosts set status='Assigned',customer_code=?,subscribe_code=?,circuit_id=?, dhcp_identifier=?, dhcp_identifier_type=2 where pool_name= ? and status='Available'  order by host_id limit 1`, cust, subs, circuit, dhcpid, pool)
		if err != nil {
			log.Info(err)
		}
		rows, _ := result.RowsAffected()
		if err != nil {
			log.Info("Problem assigning IP address")
		} else if rows > 0 {
			success, res = dhcpGetAssign(ctx, circuit)
		} else {
			log.Fatal("Could not assign IP address")
			success = false
		}
	}

	return success, res

}

func DhcpReleaseAll(ctx context.Context, circuit string) bool {
	db := SQLConnect()
	defer db.Close()
	log.Info("requesting reservation for circuit " + circuit)
	var success bool

	result, err := db.ExecContext(ctx, `update hosts set dhcp_identifier = null, hostname="", accountcode='unassigned', customer_code=null,subscribe_code=null,circuit_id=null, status='Available' where circuit_id=?`, circuit)

	rows, _ := result.RowsAffected()
	if err != nil {
		log.Info("Problem assigning IP address")
	} else if rows > 0 {
		success = true
	} else {
		log.Info("No address resources to release!")
		success = true
	}
	return success
}

func dhcpGetAssign(ctx context.Context, circuit string) (bool, ipReservation) {
	db := SQLConnect()
	defer db.Close()
	log.Info("requesting reservation for circuit " + circuit)
	var reservation ipReservation
	var success bool
	//var id int
	sqlQuery := `select host_id,dhcp_identifier,pool_name,vlan_id,ipv4_address from hosts where circuit_id=?`
	//sqlQuery := `select host_id from hosts where circuit_id=?`

	row := db.QueryRow(sqlQuery, circuit)

	var dhcpid sql.NullString
	var v4address int64

	switch err := row.Scan(&reservation.HostID, &dhcpid, &reservation.Pool, &reservation.VlanID, &v4address); err {
	//switch err := row.Scan(&id); err {

	case sql.ErrNoRows:
		log.Info("No rows were returned")
		success = false
	case nil:
		hostidstr := strconv.FormatInt(int64(reservation.HostID), 10)
		log.Info("Existing resevation host ID is " + hostidstr)
		success = true
		if dhcpid.Valid {
			reservation.DhcpID = dhcpid.String
			log.Debug("DHCP ID is " + reservation.DhcpID)
		}
		reservation.V4Addr = ipint.Int2ip(uint32(v4address))

		sqlQuery = `select reservation_id,address,prefix_len,type,dhcp6_iaid,host_id from ipv6_reservations where host_id=?`
		rows, err := db.QueryContext(ctx, sqlQuery, hostidstr)
		if err != nil {
			log.Fatal(err)
		}
		var v6res ipv6Reservation
		var iaid sql.NullInt32
		var ip6address string

		defer rows.Close()
		for rows.Next() {
			log.Info("found ipv6 reservation")
			if err := rows.Scan(&v6res.ResID, &ip6address, &v6res.Length, &v6res.Type, &iaid, &v6res.HostID); err != nil {
				log.Fatal(err)
			} else {
				success = true
				v6res.Address = net.ParseIP(ip6address)
				if iaid.Valid {
					v6res.Iaid = int(iaid.Int32)
				}
				if v6res.Type == 0 {
					reservation.V6wan = v6res.Address
				}
				if v6res.Type == 2 {
					reservation.V6dp = v6res.Address
					reservation.V6size = v6res.Length
				}
			}
		}
		log.Info(spew.Sdump(reservation))

	default:
		panic(err)
	}
	return success, reservation

}
