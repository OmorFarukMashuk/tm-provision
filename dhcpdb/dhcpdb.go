package dhcpdb

import (
	"context"
	"database/sql"
	//"fmt"
	"errors"
	"flag"
	//	"github.com/ammario/ipint"
	"github.com/davecgh/go-spew/spew"
	_ "github.com/go-sql-driver/mysql"
	log "github.com/sirupsen/logrus"
	"net"
	"strconv"
)

var (
	SQL     *sql.DB
	SQLHost = flag.String("dhcpdb.host", "dhcp04.tor2.telmax.ca", "DHCP SQL hostname")
)

// Connect to the SQL database that stores the leases and reservations
func SQLConnect() *sql.DB {
	s := sqlServer{
		Hostname: *SQLHost,
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

// Assigns an address from the named pool and returns the address object and the success of the operation.  Will return existing allocation if already assigned.
//
func DhcpAssign(node string, pool string, subs string) (reservation Reservation, err error) {
	db := SQLConnect()
	defer db.Close()
	ctx := context.TODO()
	dhcpid := subs

	log.Info("Getting address reservation for subscriber " + subs)
	reservation, err = dhcpGetAssign(ctx, subs, pool)
	if err != nil && err.Error() != "DHCP allocation not found!" {
		log.Errorf("Problem checking for existing allocation")
		return
	} else if reservation.HostID != 0 {
		log.Info("Existing address already allocated")
		return
	} else {
		log.Infof("Allocating new address pool %v on node %v", pool, node)
		var result sql.Result
		result, err = db.ExecContext(ctx, `update hosts set status='Assigned',subscriber=?, dhcp_identifier=?, dhcp_identifier_type=2 where node= ? AND pool= ? AND status='Available'  order by host_id limit 1`, subs, dhcpid, node, pool)
		if err != nil {
			log.Info(err)
			return
		}
		rows, _ := result.RowsAffected()
		if err != nil {
			log.Info("Problem assigning IP address")
		} else if rows > 0 {
			reservation, err = dhcpGetAssign(ctx, subs, pool)
		} else {
			log.Error("Could not assign IP address")
		}
	}

	return

}

// Release a specific address
func DhcpRelease(node string, pool string, subs string) (success bool, err error) {
	db := SQLConnect()
	defer db.Close()
	ctx := context.TODO()
	//	dhcpid := subs

	log.Infof("Releasing address pool %v on node %v", pool, node)
	var result sql.Result
	result, err = db.ExecContext(ctx, `update hosts set status='Available',subscriber='', dhcp_identifier='' where node= ? AND pool= ? AND subscriber= ?  order by host_id limit 3`, node, pool, subs)
	if err != nil {
		log.Info(err)
		return
	}
	rows, _ := result.RowsAffected()
	if err != nil {
		log.Info("Problem releasing IP address")
	} else if rows > 0 {
		success = true
	}

	return

}

// Release all addresses assigned to a given subscriber.  Handy if they cancel and you want to clean up
func DhcpReleaseAll(subs string) error {
	ctx := context.TODO()
	db := SQLConnect()
	defer db.Close()
	log.Info("Releasing reservations for subscriber " + subs)

	result, err := db.ExecContext(ctx, `update hosts set dhcp_identifier = null, hostname="", subscriber='unassigned', status='Available' where subscriber=?`, subs)

	rows, _ := result.RowsAffected()
	if err != nil {
		log.Info("Problem assigning IP address")
	} else if rows > 0 {
		log.Info("Released %v resources", rows)
	} else {
		log.Info("No address resources to release!")
	}
	return err
}

// Get a specific reservation with a subscriber ID and a pool name
func dhcpGetAssign(ctx context.Context, subs string, pool string) (reservation Reservation, err error) {
	db := SQLConnect()
	defer db.Close()
	log.Info("requesting reservation for subscriber " + subs + " in pool " + pool)

	//var id int
	sqlQuery := `select host_id,dhcp6_subnet_id,dhcp_identifier,pool,node,vlan,inet_ntoa(ipv4_address) from hosts where subscriber=? AND pool=?`
	//sqlQuery := `select host_id from hosts where circuit_id=?`

	row := db.QueryRow(sqlQuery, subs, pool)

	var dhcpid sql.NullString
	var v4address string

	switch err = row.Scan(&reservation.HostID, &reservation.SubnetID, &dhcpid, &reservation.Pool, &reservation.Node, &reservation.VlanID, &v4address); err {
	//switch err := row.Scan(&id); err {

	case sql.ErrNoRows:
		log.Info("No rows were returned")
		err = errors.New("DHCP allocation not found!")
		return
	case nil:
		hostidstr := strconv.FormatInt(int64(reservation.HostID), 10)
		log.Info("Existing resevation host ID is " + hostidstr)
		if dhcpid.Valid {
			reservation.DhcpID = dhcpid.String
			log.Debug("DHCP ID is " + reservation.DhcpID)
		}
		//		reservation.V4Addr = ipint.Int2ip(uint32(v4address))
		reservation.V4Addr = net.ParseIP(v4address)
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
		log.Debug(spew.Sdump(reservation))

	default:
		log.Errorf("Problem getting DHCP allocation %v", err)
		return

	}
	return

}
