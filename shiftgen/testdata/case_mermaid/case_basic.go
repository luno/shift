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

func (v insert) Insert(ctx context.Context, tx *sql.Tx, status shift.Status) (int64, error) {
	return 0, nil
}

func (v update) Update(ctx context.Context, tx *sql.Tx, from shift.Status, to shift.Status) (int64, error) {
	return 0, nil
}
