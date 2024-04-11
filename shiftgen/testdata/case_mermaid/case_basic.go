package case_basic

import (
	"github.com/luno/reflex/rsql"
	"github.com/luno/shift"
)

var events = rsql.NewEventsTableInt("events")

type status int

const (
	CREATED status = iota
	PENDING
	FAILED
	COMPLETED
)

var fsm = shift.NewFSM(events).
	Insert(CREATED, insert{}, PENDING, FAILED).
	Update(PENDING, update{}, FAILED, COMPLETED).
	Update(FAILED, update{}).
	Update(COMPLETED, update{}).
	Build()

func (v status) ShiftStatus() int {
	return int(v)
}

func (v status) ReflexType() int {
	return int(v)
}

type insert struct{}
type update struct{}
