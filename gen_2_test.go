package shift_test

// Code generated by shiftgen at test_shift_test.go:16. DO NOT EDIT.

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"github.com/luno/shift"
)

// Insert inserts a new tests table entity. All the fields of the
// i receiver are set, as well as status, created_at and updated_at.
// The newly created entity id is returned on success or an error.
func (一 i) Insert(ctx context.Context, tx *sql.Tx, st shift.Status) (int64, error) {
	var (
		q    strings.Builder
		args []interface{}
	)

	q.WriteString("insert into tests set `status`=?, `created_at`=?, `updated_at`=? ")
	args = append(args, st.ShiftStatus(), time.Now(), time.Now())

	q.WriteString(", `i1`=?")
	args = append(args, 一.I1)

	q.WriteString(", `i2`=?")
	args = append(args, 一.I2)

	q.WriteString(", `i3`=?")
	args = append(args, 一.I3)

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

// Update updates the status of a tests table entity. All the fields of the
// u receiver are updated, as well as status and updated_at.
// The entity id is returned on success or an error.
func (一 u) Update(ctx context.Context, tx *sql.Tx, from shift.Status,
	to shift.Status,
) (int64, error) {
	var (
		q    strings.Builder
		args []interface{}
	)

	q.WriteString("update tests set `status`=?, `updated_at`=? ")
	args = append(args, to.ShiftStatus(), time.Now())

	q.WriteString(", `u1`=?")
	args = append(args, 一.U1)

	q.WriteString(", `u2`=?")
	args = append(args, 一.U2)

	q.WriteString(", `u3`=?")
	args = append(args, 一.U3)

	q.WriteString(", `u4`=?")
	args = append(args, 一.U4)

	q.WriteString(", `u5`=?")
	args = append(args, 一.U5)

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
		return 0, errors.Wrap(shift.ErrRowCount, "u", j.KV("count", n))
	}

	return 一.ID, nil
}
