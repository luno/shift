package shift_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/jettison/jtest"
	"github.com/luno/reflex/rsql"
	"github.com/stretchr/testify/require"

	"github.com/luno/shift"
)

// vInsert is a versioned inserter (equivalent to shiftgen -version output).
type vInsert struct {
	Name string
}

func (一 vInsert) Insert(
	ctx context.Context, tx *sql.Tx, st shift.Status,
) (int64, error) {
	var (
		q    strings.Builder
		args []interface{}
	)

	q.WriteString("insert into versioned_users set `status`=?, `created_at`=?, `updated_at`=?, `version`=1 ")
	args = append(args, st.ShiftStatus(), time.Now(), time.Now())

	q.WriteString(", `name`=?")
	args = append(args, 一.Name)

	res, err := tx.ExecContext(ctx, q.String(), args...)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	return id, nil
}

// vUpdate is a versioned updater (equivalent to shiftgen -version output).
type vUpdate struct {
	ID   int64
	Name string
}

func (一 vUpdate) Update(
	ctx context.Context, tx *sql.Tx, from shift.Status, to shift.Status,
) (int64, error) {
	var (
		q    strings.Builder
		args []interface{}
	)

	q.WriteString("update versioned_users set `status`=?, `updated_at`=?, `version`=`version`+1 ")
	args = append(args, to.ShiftStatus(), time.Now())

	q.WriteString(", `name`=?")
	args = append(args, 一.Name)

	q.WriteString(" where `id`=? and `status`=?")
	args = append(args, 一.ID, from.ShiftStatus())

	res, err := tx.ExecContext(ctx, q.String(), args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n != 1 {
		return 0, errors.Wrap(shift.ErrRowCount, "vUpdate", j.KV("count", n))
	}

	return 一.ID, nil
}

func TestVersionInsertAndUpdate(t *testing.T) {
	dbc := setup(t)
	ctx := context.Background()

	vEvents := rsql.NewEventsTableInt("versioned_events",
		rsql.WithEventMetadataField("metadata"),
		rsql.WithoutEventsCache(),
	)

	vFSM := shift.NewFSM(vEvents, shift.WithVersion("versioned_users")).
		Insert(StatusInit, vInsert{}, StatusUpdate).
		Update(StatusUpdate, vUpdate{}).
		Build()

	// Insert
	id, err := vFSM.Insert(ctx, dbc, vInsert{Name: "alice"})
	jtest.RequireNil(t, err)
	require.Equal(t, int64(1), id)

	// Check entity version is 1
	var version int64
	err = dbc.QueryRow("select `version` from versioned_users where id=?", id).Scan(&version)
	jtest.RequireNil(t, err)
	require.Equal(t, int64(1), version)

	// Check event metadata contains version
	sc, err := vEvents.ToStream(dbc)(ctx, "")
	jtest.RequireNil(t, err)

	e, err := sc.Recv()
	jtest.RequireNil(t, err)
	require.NotEmpty(t, e.MetaData)

	v, err := shift.DecodeVersion(e.MetaData)
	jtest.RequireNil(t, err)
	require.Equal(t, int64(1), v)

	// Update
	err = vFSM.Update(ctx, dbc, StatusInit, StatusUpdate, vUpdate{ID: id, Name: "bob"})
	jtest.RequireNil(t, err)

	// Check entity version is 2
	err = dbc.QueryRow("select `version` from versioned_users where id=?", id).Scan(&version)
	jtest.RequireNil(t, err)
	require.Equal(t, int64(2), version)

	// Check second event metadata contains version 2
	e, err = sc.Recv()
	jtest.RequireNil(t, err)
	require.NotEmpty(t, e.MetaData)

	v, err = shift.DecodeVersion(e.MetaData)
	jtest.RequireNil(t, err)
	require.Equal(t, int64(2), v)
}

func TestDecodeVersionMissing(t *testing.T) {
	_, err := shift.DecodeVersion([]byte(`{}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "record_version header not found")
}

func TestDecodeVersionInvalid(t *testing.T) {
	_, err := shift.DecodeVersion([]byte(`not json`))
	require.Error(t, err)
}
