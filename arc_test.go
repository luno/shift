package shift_test

import (
	"context"
	"testing"
	"time"

	"github.com/luno/jettison/jtest"
	"github.com/stretchr/testify/require"

	"github.com/luno/shift"
)

//go:generate go run github.com/luno/shift/shiftgen -inserters=insert2 -updaters=move -table=users -out=gen_4_test.go

type insert2 struct {
	Name        string
	DateOfBirth time.Time `shift:"dob"` // Override column name.
	Amount      Currency
}

type move struct {
	ID int64
}

// afsm defines an ArcFSM with two ways to initialise an entry
// (with insert{} or insert2{}) as well as being able to move
// back Init from Update via move{}.
var afsm = shift.NewArcFSM(events).
	Insert(StatusInit, insert{}).
	Insert(StatusInit, insert2{}).
	Update(StatusInit, StatusUpdate, move{}).
	Update(StatusUpdate, StatusInit, move{}).
	Build()

func TestArcFSM(t *testing.T) {
	dbc := setup(t)

	t0 := time.Now().Truncate(time.Second)
	amount := Currency{Valid: true, Amount: 99}
	ctx := context.Background()

	// Init model
	id1, err := afsm.Insert(ctx, dbc, StatusInit, insert{Name: "insert", DateOfBirth: t0})
	jtest.RequireNil(t, err)
	require.Equal(t, int64(1), id1)

	// Move to Updated
	err = afsm.Update(ctx, dbc, StatusInit, StatusUpdate, move{ID: id1})
	jtest.RequireNil(t, err)

	// Move back to Init
	err = afsm.Update(ctx, dbc, StatusUpdate, StatusInit, move{ID: id1})
	jtest.RequireNil(t, err)

	assertUser(t, dbc, events.ToStream(dbc), usersTable, id1, "insert", t0, Currency{}, 1, 2, 1)

	// Init another model
	id2, err := afsm.Insert(ctx, dbc, StatusInit, insert2{Name: "insert2", DateOfBirth: t0, Amount: amount})
	jtest.RequireNil(t, err)
	require.Equal(t, int64(2), id2)

	assertUser(t, dbc, events.ToStream(dbc), usersTable, id2, "insert2", t0, amount, 1)
}
