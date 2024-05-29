package case_basic

import (
	"context"
	"database/sql"
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

func (v insert) Insert(ctx context.Context, tx *sql.Tx, status shift.Status) (int64, error) {
	return 0, nil
}

func (v update) Update(ctx context.Context, tx *sql.Tx, from shift.Status, to shift.Status) (int64, error) {
	return 0, nil
}
