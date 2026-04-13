package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/luno/jettison/jtest"
	"github.com/sebdah/goldie/v2"
	"github.com/stretchr/testify/require"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "lowercase unchanged", input: "hello", want: "hello"},
		{name: "camel to snake", input: "CamelCase", want: "camel_case"},
		{name: "multiple words", input: "MyFieldName", want: "my_field_name"},
		{name: "acronym", input: "IDField", want: "id_field"},
		{name: "already snake", input: "snake_case", want: "snake_case"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toSnakeCase(tt.input)
			if got != tt.want {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIDZeroValue(t *testing.T) {
	tests := []struct {
		name   string
		idType string
		want   string
	}{
		{name: "int64", idType: "int64", want: "0"},
		{name: "string", idType: "string", want: `""`},
		{name: "unknown type", idType: "uuid", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := Struct{IDType: tt.idType}
			got := s.IDZeroValue()
			if got != tt.want {
				t.Errorf("IDZeroValue() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseUpdaters(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  []string
	}{
		{name: "empty", value: "", want: nil},
		{name: "single", value: "UpdateReq", want: []string{"UpdateReq"}},
		{name: "multiple", value: "UpdateReq,CompleteReq", want: []string{"UpdateReq", "CompleteReq"}},
		{name: "with spaces", value: " UpdateReq , CompleteReq ", want: []string{"UpdateReq", "CompleteReq"}},
		{name: "whitespace only", value: "   ", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orig := *updaters
			*updaters = tt.value
			t.Cleanup(func() { *updaters = orig })

			got := parseUpdaters()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseInserters(t *testing.T) {
	tests := []struct {
		name         string
		inserterVal  string
		insertersVal string
		want         []string
		wantErr      bool
	}{
		{name: "empty", inserterVal: "", insertersVal: "", want: nil},
		{name: "single inserter", inserterVal: "InsertReq", insertersVal: "", want: []string{"InsertReq"}},
		{name: "multiple inserters", inserterVal: "", insertersVal: "InsertA,InsertB", want: []string{"InsertA", "InsertB"}},
		{name: "inserters with spaces", inserterVal: "", insertersVal: " InsertA , InsertB ", want: []string{"InsertA", "InsertB"}},
		{name: "both set returns error", inserterVal: "InsertReq", insertersVal: "InsertA", want: nil, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			origInserter := *inserter
			origInserters := *inserters
			*inserter = tt.inserterVal
			*inserters = tt.insertersVal
			t.Cleanup(func() {
				*inserter = origInserter
				*inserters = origInserters
			})

			got, err := parseInserters()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			jtest.RequireNil(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

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

func TestMermaid(t *testing.T) {
	cc := []struct {
		dir     string
		outFile string
	}{
		{
			dir:     "case_mermaid",
			outFile: "shift_gen.mmd",
		},
		{
			dir:     "case_mermaid_arcfsm",
			outFile: "shift_gen.mmd",
		},
	}

	for _, c := range cc {
		t.Run(c.dir, func(t *testing.T) {
			err := os.Setenv("GOFILE", "shiftgen_test.go")
			jtest.RequireNil(t, err)
			err = os.Setenv("GOLINE", "123")
			jtest.RequireNil(t, err)

			bb, err := generateMermaidDiagram(filepath.Join("testdata", c.dir))

			jtest.RequireNil(t, err)
			g := goldie.New(t)
			g.Assert(t, filepath.Join(c.dir, c.outFile), []byte(bb))
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
