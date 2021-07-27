package shift

import (
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/luno/jettison/errors"
)

// TODO(corver): Implement TestArcFSM

// TestFSM tests the provided FSM instance by driving it through all possible
// state transitions using fuzzed data. It ensures all states are reachable and
// that the sql queries match the schema.
func TestFSM(_ testing.TB, dbc *sql.DB, fsm *FSM) error {
	if fsm.insertStatus == nil {
		return errors.New("fsm without insert status not supported")
	}
	found := map[int]bool{
		fsm.insertStatus.ShiftStatus(): true,
	}

	paths := buildPaths(fsm.states, fsm.insertStatus)
	for i, path := range paths {
		name := fmt.Sprintf("%d_from_%d_to_%d_len_%d", i, path[0].st, path[len(path)-1].st, len(path))
		msg := "error in path " + name

		insert, err := randomInsert(path[0].req)
		if err != nil {
			return errors.Wrap(err, msg)
		}
		id, err := fsm.Insert(context.Background(), dbc, insert)
		if err != nil {
			return errors.Wrap(err, msg)
		}

		from := path[0].st
		for _, up := range path[1:] {
			update, err := randomUpdate(up.req, id)
			if err != nil {
				return errors.Wrap(err, msg)
			}
			err = fsm.Update(context.Background(), dbc, from, up.st, update)
			if err != nil {
				return errors.Wrap(err, msg)
			}
			from = up.st
			found[up.st.ShiftStatus()] = true
		}
	}
	for st := range fsm.states {
		if !found[st] {
			return errors.New("status not reachable")
		}
	}
	return nil
}

func randomUpdate(req interface{}, id int64) (u Updater, err error) {
	u, ok := req.(Updater)
	if !ok {
		return nil, errors.New("req not of tupe Updater")
	}
	s := reflect.New(reflect.ValueOf(req).Type()).Elem()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		t := f.Type()
		if s.Type().Field(i).Name == "ID" {
			f.SetInt(id)
		} else {
			f.Set(randVal(t))
		}
	}
	return s.Interface().(Updater), nil
}

func randomInsert(req interface{}) (Inserter, error) {
	_, ok := req.(Inserter)
	if !ok {
		return nil, errors.New("req not of type Inserter")
	}

	s := reflect.New(reflect.ValueOf(req).Type()).Elem()
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		f.Set(randVal(f.Type()))
	}
	return s.Interface().(Inserter), nil
}

func buildPaths(states map[int]status, from Status) [][]status {
	var res [][]status
	here := states[from.ShiftStatus()]
	hasEnd := len(here.next) == 0
	delete(states, from.ShiftStatus()) // Break cycles
	for next := range here.next {
		if _, ok := states[next.ShiftStatus()]; !ok {
			hasEnd = true // Stop at breaks
			continue
		}
		paths := buildPaths(states, next)
		for _, path := range paths {
			res = append(res, append([]status{here}, path...))
		}
	}
	states[from.ShiftStatus()] = here
	if hasEnd {
		res = append(res, []status{here})
	}
	return res
}

var (
	intType        = reflect.TypeOf((int)(0))
	int64Type      = reflect.TypeOf((int64)(0))
	float64Type    = reflect.TypeOf((float64)(0))
	timeType       = reflect.TypeOf(time.Time{})
	sliceByteType  = reflect.TypeOf([]byte(nil))
	boolType       = reflect.TypeOf(false)
	stringType     = reflect.TypeOf("")
	nullTimeType   = reflect.TypeOf(sql.NullTime{})
	nullStringType = reflect.TypeOf(sql.NullString{})
)

func randVal(t reflect.Type) reflect.Value {
	var v interface{}
	switch t {
	case intType:
		v = rand.Intn(1000)
	case int64Type:
		v = int64(rand.Intn(1000))
	case float64Type:
		v = rand.Float64() * 1000
	case timeType:
		d := time.Duration(rand.Intn(1000)) * time.Hour
		v = time.Now().Add(-d)
	case sliceByteType:
		v = randBytes(rand.Intn(64))
	case boolType:
		v = rand.Float64() < 0.5
	case stringType:
		v = hex.EncodeToString(randBytes(rand.Intn(10)))
	case nullTimeType:
		v = sql.NullTime{
			Valid: rand.Float64() < 0.5,
			Time:  time.Now(),
		}
	case nullStringType:
		v = sql.NullString{
			Valid:  rand.Float64() < 0.5,
			String: hex.EncodeToString(randBytes(rand.Intn(10))),
		}
	default:
		return reflect.Indirect(reflect.New(t))
	}
	return reflect.ValueOf(v)
}

func randBytes(size int) []byte {
	b := make([]byte, size)
	rand.Read(b)
	return b
}
