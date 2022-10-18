package main

import (
	"github.com/luno/jettison/jtest"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
)

func TestGen(t *testing.T) {
	cc := []struct {
		dir       string
		table     string
		inserters []string
		updaters  []string
		stringID  bool
		outFile   string
	}{
		{
			dir:       "case_basic",
			table:     "users",
			inserters: []string{"insert"},
			updaters:  []string{"update", "complete"},
			outFile:   "shift_gen.go",
		},
		{
			dir:       "case_specify_times",
			table:     "foo",
			inserters: []string{"iFoo"},
			updaters:  []string{"uFoo"},
			outFile:   "shift_gen.go",
		},
		{
			dir:       "case_special_names",
			table:     "bar_baz",
			inserters: []string{"类型"},
			updaters:  []string{"변수", "エラー"},
			outFile:   "shift_gen.go",
		},
		{
			dir:       "case_basic_string",
			table:     "users",
			inserters: []string{"insert"},
			updaters:  []string{"update", "complete"},
			stringID:  true,
			outFile:   "shift_gen.go",
		},
	}

	for _, c := range cc {
		t.Run(c.dir, func(t *testing.T) {
			err := os.Setenv("GOFILE", "shiftgen_test.go")
			jtest.RequireNil(t, err)
			err = os.Setenv("GOLINE", "123")
			jtest.RequireNil(t, err)

			bb, err := generateSrc(
				filepath.Join("testdata", c.dir),
				c.table, c.inserters, c.updaters, "status",
				filepath.Join("testdata", c.dir, c.outFile))

			jtest.RequireNil(t, err)
			g := goldie.New(t)
			g.Assert(t, filepath.Join(c.dir, c.outFile), bb)
		})
	}
}

func TestGenFailure(t *testing.T) {
	cc := []struct {
		dir       string
		table     string
		inserters []string
		updaters  []string
		stringID  bool
		outFile   string
		outErr    error
	}{
		{
			dir:       "case_id_insert_mismatch",
			table:     "users",
			inserters: []string{"insert"},
			updaters:  []string{"complete"},
			outFile:   "shift_gen.go",
			outErr:    ErrIDTypeMismatch,
		},
		{
			dir:       "case_id_update_mismatch",
			table:     "users",
			inserters: []string{"insert"},
			updaters:  []string{"update", "complete"},
			outFile:   "shift_gen.go",
			outErr:    ErrIDTypeMismatch,
		},
	}

	for _, c := range cc {
		t.Run(c.dir, func(t *testing.T) {
			_, err := generateSrc(
				filepath.Join("testdata", "failure", c.dir),
				c.table, c.inserters, c.updaters, "status",
				filepath.Join("testdata", "failure", c.dir, c.outFile))

			require.EqualError(t, err, c.outErr.Error())
		})
	}
}
