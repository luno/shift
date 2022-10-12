package gen_str_test

import (
	"context"
	"github.com/luno/jettison/jtest"
	"testing"
	"time"

	"github.com/luno/reflex/rsql"
	"github.com/luno/shift"
)

//go:generate shiftgen -inserter=insert -updaters=update,complete -table=users -out=gen_test.go -primary_type=string
// Also support non-auto-incrementing int64 primary keys
////go:generate shiftgen ... -primary_type=int64 -auto_increment=false

type insert struct {
	KSUID       string `shift:"ksuid,primary"` // primary required if default of ID not used/wanted.
	Name        string
	DateOfBirth time.Time `shift:"dob"` // Override column name.
}

type update struct {
	KSUID  string `shift:"ksuid,primary"`
	Name   string
	Amount Currency
}

type complete struct {
	KSUID string `shift:"ksuid,primary"`
}

type TestStatus int

func (s TestStatus) ShiftStatus() int {
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

var events = rsql.NewEventsTable("events")
var fsm = shift.NewGenericFSM[string](events).
	Insert(StatusInit, insert{}, StatusUpdate).
	Update(StatusUpdate, update{}, StatusComplete).
	Update(StatusComplete, complete{}).
	Build()

func TestFSM(t *testing.T) {
	ctx := context.Background()
	dbc := setup(t)

	ksuid := "0ujsswThIBTUYm2u8FjO3fXtY1K"
	err := fsm.Insert(ctx, dbc, insert{ksuid, "John", time.Now()})
	jtest.AssertNil(t, err)

	err = fsm.Update(ctx, dbc, StatusInit, StatusUpdate, update{ksuid, "Jane", Currency{true, 10}})
	jtest.AssertNil(t, err)

	err = fsm.Update(ctx, dbc, StatusUpdate, StatusComplete, complete{ksuid})
	jtest.AssertNil(t, err)
}
