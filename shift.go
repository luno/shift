// Package shift provides the persistence layer for a simple "finite state machine"
// domain model with validation, explicit fields and reflex events per state change.
//
// shift.NewFSM builds a FSM instance that allows specific mutations of
// the domain model in the underlying sql table via inserts and updates.
// All mutations update the status of the model, mutates some fields and
// inserts a reflex event.
package shift

import (
	"context"
	"database/sql"
	"reflect"

	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
)

// Status is an individual state in the FSM.
//
// The canonical implementation is:
//	type MyStatus int
//	func (s MyStatus) ShiftStatus() {}
//	func (s MyStatus) Enum() int {
//		return int(s)
//	}
//  // ReflexType is an optional method to override the default reflex type.
//  func (s MyStatus) ReflexType() reflex.EventType {}
//	const (
//		StatusUnknown MyStatus = 0
//		StatusInsert  MyStatus = 1
//	)
type Status interface {
	Enum() int
	ShiftStatus()
	ReflexType() int
}

type Inserter interface {
	Insert(context.Context, *sql.Tx, Status) (int64, error)
}
type Updater interface {
	Update(context.Context, *sql.Tx, Status, Status) (int64, error)
}

type FSM struct {
	events       rsql.EventsTableInt
	states       map[Status]status
	insertStatus Status
}

// Insert returns the id of the newly inserted domain model.
func (fsm *FSM) Insert(ctx context.Context, dbc *sql.DB, inserter Inserter) (int64, error) {
	var (
		st = fsm.insertStatus
	)
	if !sameType(fsm.states[st].req, inserter) {
		return 0, errMismatchStatusReq
	}

	tx, err := dbc.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	id, err := inserter.Insert(ctx, tx, st)
	if err != nil {
		return 0, err
	}

	notify, err := fsm.events.Insert(ctx, tx, id, fsm.states[st].t)
	if err != nil {
		return 0, err
	}
	defer notify()

	return id, tx.Commit()
}

func (fsm *FSM) Update(ctx context.Context, dbc *sql.DB, from Status, to Status, updater Updater) error {
	tx, err := dbc.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	notify, err := fsm.UpdateTx(ctx, tx, from, to, updater)
	if err != nil {
		return err
	}
	defer notify()

	return tx.Commit()
}

func (fsm *FSM) UpdateTx(ctx context.Context, tx *sql.Tx, from Status, to Status, updater Updater) (rsql.NotifyFunc, error) {
	t, ok := fsm.states[to]
	if !ok {
		return nil, errUnknownStatus
	}
	if !sameType(t.req, updater) {
		return nil, errMismatchStatusReq
	}
	f, ok := fsm.states[from]
	if !ok {
		return nil, errUnknownStatus
	} else if !f.next[to] {
		return nil, errInvalidTransition
	}

	id, err := updater.Update(ctx, tx, from, to)
	if err != nil {
		return nil, err
	}

	return fsm.events.Insert(ctx, tx, id, fsm.states[to].t)
}

type status struct {
	st     Status
	t      reflex.EventType
	req    interface{}
	insert bool
	next   map[Status]bool
}

func sameType(a interface{}, b interface{}) bool {
	return reflect.TypeOf(a) == reflect.TypeOf(b)
}
