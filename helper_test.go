package shift_test

import (
	"database/sql"
	"database/sql/driver"
	"flag"
	"log"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

var schemas = []string{`
  create temporary table users (
    id bigint not null auto_increment,
    name varchar(255) not null,
    dob datetime not null,
    amount varchar(255),

    status     tinyint not null,
    created_at datetime not null,
    updated_at datetime not null,

    primary key (id)
  );`, `
  create temporary table events (
    id bigint not null auto_increment,
    foreign_id bigint not null,
    timestamp datetime not null,
    type tinyint not null,
    metadata blob,

    primary key (id)
  );`, `
  create temporary table tests (
    id bigint not null auto_increment,
    i1 bigint not null, 
    i2 varchar(255) not null,
    i3 datetime not null,
    u1 bool,
    u2 varchar(255),
    u3 datetime,
    u4 varchar(255),
    u5 binary(64),
    
    status     tinyint not null,
    created_at datetime not null,
    updated_at datetime not null,
    
    primary key (id)
  );`}

// TODO(corver): Refactor this to use sqllite.
var dbTestURI = flag.String("db_test_base", "root@unix("+getSocketFile()+")/test?", "Test database uri")

func getSocketFile() string {
	sock := "/tmp/mysql.sock"
	if _, err := os.Stat(sock); os.IsNotExist(err) {
		// try common linux/Ubuntu socket file location
		return "/var/run/mysqld/mysqld.sock"
	}
	return sock
}

func connect() (*sql.DB, error) {
	str := *dbTestURI + "parseTime=true&collation=utf8mb4_general_ci"
	dbc, err := sql.Open("mysql", str)
	if err != nil {
		return nil, err
	}

	dbc.SetMaxOpenConns(1)

	if _, err := dbc.Exec("set time_zone='+00:00';"); err != nil {
		log.Fatalf("error setting db time_zone: %v", err)
	}

	return dbc, nil
}

func setup(t *testing.T) *sql.DB {
	dbc, err := connect()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, dbc.Close()) })

	for _, s := range schemas {
		_, err := dbc.Exec(s)
		require.NoError(t, err)
	}

	return dbc
}

// Currency is a custom "currency" type stored a string in the DB.
type Currency struct {
	Valid  bool
	Amount int64
}

func (c *Currency) Scan(src interface{}) error {
	var s sql.NullString
	if err := s.Scan(src); err != nil {
		return err
	}
	if !s.Valid {
		*c = Currency{
			Valid:  false,
			Amount: 0,
		}
		return nil
	}
	i, err := strconv.ParseInt(s.String, 10, 64)
	if err != nil {
		return err
	}
	*c = Currency{
		Valid:  true,
		Amount: i,
	}
	return nil
}

func (c Currency) Value() (driver.Value, error) {
	return strconv.FormatInt(c.Amount, 10), nil
}
