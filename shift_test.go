package shift_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
	"github.com/luno/shift"
	"github.com/stretchr/testify/require"
)

//go:generate shiftgen -inserter=insert -updaters=update,complete -table=users -out=gen_1_test.go

type insert struct {
	Name        string
	DateOfBirth time.Time `shift:"dob"` // Override column name.
}

type update struct {
	ID     int64
	Name   string
	Amount Currency
}

type complete struct {
	ID int64
}

type TestStatus int

func (s TestStatus) ShiftStatus() {}

func (s TestStatus) Enum() int {
	return int(s)
}

func (s TestStatus) ReflexType() int {
	return int(s)
}

const (
	StatusInit     TestStatus = 1
	StatusUpdate   TestStatus = 2
	StatusComplete TestStatus = 3
)

var events = rsql.NewEventsTableInt("events")
var fsm = shift.NewFSM(events).
	Insert(StatusInit, insert{}, StatusUpdate).
	Update(StatusUpdate, update{}, StatusComplete).
	Update(StatusComplete, complete{}).
	Build()

func TestAboveFSM(t *testing.T) {
	dbc := setup(t)
	defer dbc.Close()

	require.NoError(t, shift.TestFSM(t, dbc, fsm))
}

func TestBasic(t *testing.T) {
	dbc := setup(t)
	defer dbc.Close()

	t0 := time.Now().Truncate(time.Second)
	amount := Currency{Valid: true, Amount: 99}
	ctx := context.Background()

	// Init model
	id, err := fsm.Insert(ctx, dbc, insert{Name: "insertMe", DateOfBirth: t0})
	require.NoError(t, err)
	require.Equal(t, int64(1), id)

	assertUser(t, dbc, events.ToStream(dbc), id, "insertMe", t0, Currency{}, 1)

	// Update model
	err = fsm.Update(ctx, dbc, StatusInit, StatusUpdate, update{ID: id, Name: "updateMe", Amount: amount})
	require.NoError(t, err)

	assertUser(t, dbc, events.ToStream(dbc), id, "updateMe", t0, amount, 1, 2)

	// Complete model
	err = fsm.Update(ctx, dbc, StatusUpdate, StatusComplete, complete{ID: id})
	require.NoError(t, err)

	assertUser(t, dbc, events.ToStream(dbc), id, "updateMe", t0, amount, 1, 2, 3)
}

func assertUser(t *testing.T, dbc *sql.DB, stream reflex.StreamFunc, id int64,
	exName string, exDOB time.Time, exAmount Currency, exEvents ...TestStatus) {
	var name sql.NullString
	var amount Currency
	var dob time.Time
	err := dbc.QueryRow("select name, dob, amount "+
		"from users where id=?", id).Scan(&name, &dob, &amount)
	require.NoError(t, err)
	require.Equal(t, exName, name.String)
	require.Equal(t, exDOB.UTC(), dob.UTC())
	require.Equal(t, exAmount, amount)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sc, err := stream(ctx, "")
	require.NoError(t, err)
	for _, exE := range exEvents {
		e, err := sc.Recv()
		require.NoError(t, err)
		require.Equal(t, int(exE), e.Type.ReflexType())
	}

}
