package shift_test

// Code generated by shiftgen at arc_test.go:14. DO NOT EDIT.

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/shift"
)

// Insert inserts a new users table entity. All the fields of the
// insert2 receiver are set, as well as status, created_at and updated_at.
// The newly created entity id is returned on success or an error.
func (一 insert2) Insert(
	ctx context.Context, tx *sql.Tx, st shift.Status,
) (int64, error) {
	var (
		q    strings.Builder
		args []interface{}
	)

	q.WriteString("insert into users set `status`=?, `created_at`=?, `updated_at`=? ")
	args = append(args, st.ShiftStatus(), time.Now(), time.Now())

	q.WriteString(", `name`=?")
	args = append(args, 一.Name)

	q.WriteString(", `dob`=?")
	args = append(args, 一.DateOfBirth)

	q.WriteString(", `amount`=?")
	args = append(args, 一.Amount)

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

// Update updates the status of a users table entity. All the fields of the
// move receiver are updated, as well as status and updated_at.
// The entity id is returned on success or an error.
func (一 move) Update(
	ctx context.Context, tx *sql.Tx, from shift.Status, to shift.Status,
) (int64, error) {
	var (
		q    strings.Builder
		args []interface{}
	)

	q.WriteString("update users set `status`=?, `updated_at`=? ")
	args = append(args, to.ShiftStatus(), time.Now())

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
		return 0, errors.Wrap(shift.ErrRowCount, "move", j.KV("count", n))
	}

	return 一.ID, nil
}
