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

var fsm = shift.NewArcFSM(events).
	Insert(CREATED, insert{}).
	Update(CREATED, FAILED, update{}).
	Update(CREATED, PENDING, update{}).
	Update(PENDING, FAILED, update{}).
	Update(PENDING, COMPLETED, update{}).
	Build()

func (v status) ShiftStatus() int {
	return int(v)
}

func (v status) ReflexType() int {
	return int(v)
}

type insert struct{}
type update struct{}
