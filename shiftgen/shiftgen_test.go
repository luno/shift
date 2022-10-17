package main

import (
	"github.com/luno/jettison/jtest"
	"github.com/sebdah/goldie/v2"
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
