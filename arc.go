package shift

import (
	"context"
	"database/sql"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/reflex"
	"github.com/luno/reflex/rsql"
)

// NewArcFSM returns a new ArcFSM builder.
func NewArcFSM(events eventInserter, opts ...option) arcbuilder {
	fsm := ArcFSM{
		updates: make(map[int][]tuple),
		events:  events,
	}

	for _, opt := range opts {
		opt(&fsm.options)
	}

	return arcbuilder(fsm)
}

type arcbuilder ArcFSM

func (b arcbuilder) Insert(st Status, inserter Inserter) arcbuilder {
	b.inserts = append(b.inserts, tuple{
		Status: st.ShiftStatus(),
		Type:   inserter,
	})
	return b
}

func (b arcbuilder) Update(from, to Status, updater Updater) arcbuilder {
	tups := b.updates[from.ShiftStatus()]

	tups = append(tups, tuple{
		Status: to.ShiftStatus(),
		Type:   updater,
	})

	b.updates[from.ShiftStatus()] = tups

	return b
}

func (b arcbuilder) Build() *ArcFSM {
	fsm := ArcFSM(b)
	return &fsm
}

type tuple struct {
	Status int
	Type   interface{}
}

// ArcFSM is a defined Finite-State-Machine that allows specific mutations of
// the domain model in the underlying sql table via inserts and updates.
// All mutations update the status of the model, mutates some fields and
// inserts a reflex event.
//
// ArcFSM doesn't have the restriction of FSM and can be defined with arbitrary transitions.
type ArcFSM struct {
	options
	events  eventInserter
	inserts []tuple
	updates map[int][]tuple
}

func (fsm *ArcFSM) Insert(ctx context.Context, dbc *sql.DB, st Status, inserter Inserter) (int64, error) {
	tx, err := dbc.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	id, notify, err := fsm.InsertTx(ctx, tx, st, inserter)
	if err != nil {
		return 0, err
	}
	defer notify()

	return id, tx.Commit()
}

func (fsm *ArcFSM) InsertTx(ctx context.Context, tx *sql.Tx, st Status, inserter Inserter) (int64, rsql.NotifyFunc, error) {
	var found bool
	for _, tup := range fsm.inserts {
		if tup.Status == st.ShiftStatus() && sameType(tup.Type, inserter) {
			found = true
			break
		}
	}
	if !found {
		return 0, nil, errors.Wrap(ErrInvalidStateTransition, "invalid insert status and inserter", j.KV("status", st.ShiftStatus()))
	}

	return insertTx(ctx, tx, st, inserter, fsm.events, reflex.EventType(st), fsm.options)
}

func (fsm *ArcFSM) Update(ctx context.Context, dbc *sql.DB, from, to Status, updater Updater) error {
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

func (fsm *ArcFSM) UpdateTx(ctx context.Context, tx *sql.Tx, from, to Status, updater Updater) (rsql.NotifyFunc, error) {
	tl, ok := fsm.updates[from.ShiftStatus()]
	if !ok {
		return nil, errors.Wrap(ErrInvalidStateTransition, "invalid update from status", j.KV("status", from.ShiftStatus()))
	}

	var found bool
	for _, tup := range tl {
		if tup.Status == to.ShiftStatus() && sameType(tup.Type, updater) {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.Wrap(ErrInvalidStateTransition, "invalid update to status and updater", j.KV("status", from.ShiftStatus()))
	}

	return updateTx(ctx, tx, from, to, updater, fsm.events, reflex.EventType(to), fsm.options)
}
