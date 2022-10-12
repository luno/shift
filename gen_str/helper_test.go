package gen_str_test

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
    ksuid varchar(50) not null,
    name varchar(255) not null,
    dob datetime not null,
    amount varchar(255),

    status     tinyint not null,
    created_at datetime not null,
    updated_at datetime not null,

    primary key (ksuid)
  );`, `
  create temporary table events (
    id bigint not null auto_increment,
    foreign_id varchar(50) not null,
    timestamp datetime not null,
    type tinyint not null,
    metadata blob,

    primary key (id)
  );`}

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
	dbc, err := sql.Open("mysql", *dbTestURI+"parseTime=true&collation=utf8mb4_general_ci")
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

func (c *Currency) Scan(src any) error {
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
