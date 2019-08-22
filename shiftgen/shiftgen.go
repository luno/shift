// Package main generates method receivers functions for structs to implement
// shift Inserter and Updater interfaces.
//
//  Usage:
//    //go:generate shiftgen -table=model_table -inserter=InsertReq -updaters=UpdateReq,CompleteReq
package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"text/template"
)

// Tag is the shiftgen struct tag that should be used to override sql column names
// for struct fields (the default is snake case of the field name).
//   Ex `shift:"custom_col_name"`.
const Tag = "shift"

const tagPrefix = "`" + Tag + ":"

var (
	updaters = flag.String("updaters", "",
		"The struct types (comma seperated) to generate Update methods for")
	inserter = flag.String("inserter", "",
		"The struct type to generate a Insert method for")
	table = flag.String("table", "",
		"The sql table name to insert and update")
	statusField = flag.String("status_field", "status",
		"The sql column in the table containing the status")
	outFile = flag.String("out", "shift_gen.go",
		"output filename")
	quoteChar = flag.String("quote_char", "`",
		"Character to use when quoting column names")
)

type Field struct {
	Name string
	Col  string
}

type Struct struct {
	Table       string
	Type        string
	StatusField string
	Fields      []Field
}

type Data struct {
	Package   string
	GenSource string
	Updaters  []Struct
	Inserter  *Struct
}

func main() {
	flag.Parse()

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	ups := make(map[string]bool)
	for _, u := range strings.Split(*updaters, ",") {
		ups[strings.TrimSpace(u)] = true
	}

	if len(ups) == 0 && *inserter == "" {
		log.Fatal("No updaters or inserter specified")
	}
	if *table == "" {
		log.Fatal("No table specified")
	}

	fs := token.NewFileSet()
	asts, err := parser.ParseDir(fs, pwd, nil, 0)
	if err != nil {
		log.Fatal(err)
	}

	data := Data{
		GenSource: os.Getenv("GOFILE") + ":" + os.Getenv("GOLINE"),
	}

	for p, a := range asts {
		ast.Inspect(a, func(n ast.Node) bool {
			t, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			typ := t.Name.Name
			isU, first := ups[typ]
			isI := *inserter == typ
			if !isU && !isI {
				return true
			}
			if isU && !first {
				log.Fatalf("Found multiple updater struct definitions: %s", typ)
			} else if isI && data.Inserter != nil {
				log.Fatalf("Found multiple inserter struct definitions: %s", typ)
			}

			if data.Package != "" && data.Package != p {
				log.Fatal("Struct types defined in seperate packages")
			}
			data.Package = p

			s, ok := t.Type.(*ast.StructType)
			if !ok {
				log.Fatalf("Found %s, but it is not a struct type", typ)
			}
			foundID := false
			st := Struct{Type: typ, Table: *table, StatusField: *statusField}
			for _, f := range s.Fields.List {
				if len(f.Names) == 0 {
					log.Fatalf("Found %s, but has anonymous field (maybe shift.Reflect)", typ)
				}
				if len(f.Names) != 1 {
					log.Fatalf("Found %s, but one field multiple names: %v", typ, f.Names)
				}
				name := f.Names[0].Name
				if name == "ID" {
					foundID = true
					// Skip ID fields for updaters (since they are hardcoded)
					continue
				}

				col := toSnakeCase(name)
				if f.Tag != nil && strings.HasPrefix(f.Tag.Value, tagPrefix) {
					col = reflect.StructTag(f.Tag.Value[1 : len(f.Tag.Value)-1]).Get(Tag) // Delete first and last quotation
				}

				field := Field{
					Col:  col,
					Name: name,
				}
				st.Fields = append(st.Fields, field)
			}
			if isU {
				if !foundID {
					log.Fatalf("Updater must contain ID field: %s", typ)
				}
				data.Updaters = append(data.Updaters, st)
				ups[typ] = false
				return true
			}

			data.Inserter = &st
			if foundID {
				log.Fatalf("Inserter may not contain ID field: %s", typ)
			}
			return true
		})
	}

	for st, missing := range ups {
		if missing {
			log.Fatalf("Couldn't find updater: %v", st)
		}
	}
	if *inserter != "" && data.Inserter == nil {
		log.Fatalf("Couldn't find inserter: %v", *inserter)
	}

	if err := writeOutput(data, pwd); err != nil {
		log.Fatalf("Error writing file: %v", err)
	}
}

func execTpl(out io.Writer, tpl string, data interface{}) error {
	t := template.New("").Funcs(map[string]interface{}{
		"col": quoteCol,
	})

	tp, err := t.Parse(tpl)
	if err != nil {
		return err
	}

	return tp.Execute(out, data)
}

func quoteCol(colName string) string {
	return *quoteChar + colName + *quoteChar
}

func writeOutput(data Data, pwd string) error {
	var out bytes.Buffer

	err := execTpl(&out, headerTpl, data)
	if err != nil {
		return err
	}
	if data.Inserter != nil {
		err := execTpl(&out, inserterTpl, *data.Inserter)
		if err != nil {
			return err
		}
	}
	for _, u := range data.Updaters {
		err := execTpl(&out, updaterTpl, u)
		if err != nil {
			return err
		}
	}

	outname := path.Join(pwd, *outFile)
	return ioutil.WriteFile(outname, out.Bytes(), 0644)
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(col string) string {
	snake := matchFirstCap.ReplaceAllString(col, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}
