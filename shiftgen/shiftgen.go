// Command shiftgen generates method receivers functions for structs to implement
// shift Inserter and Updater interfaces. The implementations insert and update
// rows in mysql.
//
// Note shiftgen does not support generating GetMetadata functions for
// MetadataInserter or MetadataUpdater since it is orthogonal to inserting
// and updating domain entity rows.
//
//	Usage:
//	  //go:generate shiftgen -table=model_table -inserter=InsertReq -updaters=UpdateReq,CompleteReq
package main

import (
	"bytes"
	"flag"
	"github.com/luno/jettison/errors"
	"github.com/luno/jettison/j"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"text/template"

	"golang.org/x/tools/imports"
)

// Tag is the shiftgen struct tag that should be used to override sql column names
// for struct fields (the default is snake case of the field name).
//
//	Ex `shift:"custom_col_name"`.
const Tag = "shift"

const tagPrefix = "`" + Tag + ":"

// idFieldName is the name of the field in the Go struct used for the table's ID
// TODO: Support custom ID field name.
const idFieldName = "ID"

var (
	updaters = flag.String("updaters", "",
		"The struct types (comma seperated) to generate Update methods for")
	inserter = flag.String("inserter", "",
		"The struct type to generate a Insert method for")
	inserters = flag.String("inserters", "",
		"The ArcFSM struct types (comma seperated) to generate Insert methods for")
	table = flag.String("table", "",
		"The sql table name to insert and update")
	statusField = flag.String("status_field", "status",
		"The sql column in the table containing the status")
	outFile = flag.String("out", "shift_gen.go",
		"output filename")
	quoteChar = flag.String("quote_char", "`",
		"Character to use when quoting column names")
)

var ErrIDTypeMismatch = errors.New("Inserters and updaters' ID fields should have matching types")

type Field struct {
	Name string
	Col  string
}

type Struct struct {
	Table           string
	Type            string
	StatusField     string
	Fields          []Field
	CustomCreatedAt bool
	CustomUpdatedAt bool
	HasID           bool
	// IDType is the type of the ID field
	IDType string
}

func (s Struct) IDZeroValue() string {
	switch s.IDType {
	case "string":
		return `""`
	case "int64":
		return `0`
	}
	return ``
}

type Data struct {
	Package   string
	GenSource string
	Updaters  []Struct
	Inserters []Struct
}

func main() {
	flag.Parse()

	ii, err := parseInserters()
	if err != nil {
		log.Fatal(err)
	}
	uu := parseUpdaters()

	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	filePath := path.Join(pwd, *outFile)

	src, err := generateSrc(pwd, *table, ii, uu, *statusField, filePath)
	if err != nil {
		log.Fatal(err)
	}

	if err = os.WriteFile(filePath, src, 0644); err != nil {
		log.Fatal(errors.Wrap(err, "Error writing file"))
	}
}

func parseInserters() ([]string, error) {
	if *inserter != "" && *inserters != "" {
		return nil, errors.New("Either define inserter or inserters, not both")
	}

	var ii []string
	if *inserter != "" {
		ii = append(ii, *inserter)
	} else if strings.TrimSpace(*inserters) != "" {
		for _, i := range strings.Split(*inserters, ",") {
			ii = append(ii, strings.TrimSpace(i))
		}
	}
	return ii, nil
}

func parseUpdaters() []string {
	var uu []string
	if strings.TrimSpace(*updaters) != "" {
		for _, u := range strings.Split(*updaters, ",") {
			uu = append(uu, strings.TrimSpace(u))
		}
	}
	return uu
}

func generateSrc(pkgPath, table string, inserters, updaters []string, statusField, filePath string) ([]byte, error) {
	if table == "" {
		return nil, errors.New("No table specified")
	}
	if len(inserters) == 0 && len(updaters) == 0 {
		return nil, errors.New("No inserter or updaters specified")
	}

	fs := token.NewFileSet()
	asts, err := parser.ParseDir(fs, pkgPath, nil, 0)
	if err != nil {
		return nil, err
	}

	data := Data{
		GenSource: os.Getenv("GOFILE") + ":" + os.Getenv("GOLINE"),
	}

	ins := make(map[string]bool, len(inserters))
	for _, i := range inserters {
		ins[i] = true
	}
	ups := make(map[string]bool, len(updaters))
	for _, u := range updaters {
		ups[u] = true
	}
	for p, a := range asts {
		var inspectErr error
		ast.Inspect(a, func(n ast.Node) bool {
			if inspectErr != nil {
				return false
			}

			t, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}
			typ := t.Name.Name
			isU, firstU := ups[typ]
			isI, firstI := ins[typ]
			if !isU && !isI {
				return true
			}

			if isU && !firstU {
				log.Fatalf("Found multiple updater struct definitions: %s", typ)
			}
			if isI && !firstI {
				log.Fatalf("Found multiple inserter struct definitions: %s", typ)
			}

			if data.Package != "" && data.Package != p {
				inspectErr = errors.New("Struct types defined in separate packages")
			}
			data.Package = p

			s, ok := t.Type.(*ast.StructType)
			if !ok {
				inspectErr = errors.New("Inserter/updater must be a struct type", j.MKV{"name": typ})
			}
			st := Struct{Type: typ, Table: table, StatusField: statusField, IDType: "int64"}
			for _, f := range s.Fields.List {
				if len(f.Names) == 0 {
					inspectErr = errors.New("Inserter/updater, but has anonymous field (maybe shift.Reflect)", j.MKV{"name": typ})
				}
				if len(f.Names) != 1 {
					inspectErr = errors.New("Inserter/updaters, but one field multiple names: %v", j.MKV{"name": typ, "field_names": f.Names})
				}
				name := f.Names[0].Name
				if name == idFieldName {
					st.HasID = true
					if ti, ok := f.Type.(*ast.Ident); !ok {
						inspectErr = errors.New("ID field should be of type int64 or string")
					} else {
						st.IDType = ti.Name
					}
					// Skip ID fields for updaters (since they are hardcoded)
					continue
				}

				col := toSnakeCase(name)
				if f.Tag != nil && strings.HasPrefix(f.Tag.Value, tagPrefix) {
					col = reflect.StructTag(f.Tag.Value[1 : len(f.Tag.Value)-1]).Get(Tag) // Delete first and last quotation
				}

				if col == "created_at" {
					st.CustomCreatedAt = true
				}

				if col == "updated_at" {
					st.CustomUpdatedAt = true
				}

				field := Field{
					Col:  col,
					Name: name,
				}
				st.Fields = append(st.Fields, field)
			}
			if isU {
				if !st.HasID {
					inspectErr = errors.New("Updater must contain ID field", j.MKV{"field": typ})
				}
				data.Updaters = append(data.Updaters, st)
				ups[typ] = false
			} else {
				data.Inserters = append(data.Inserters, st)
				ins[typ] = false
			}

			return true
		})
		if inspectErr != nil {
			return nil, inspectErr
		}
	}

	for in, missing := range ins {
		if missing {
			return nil, errors.New("Couldn't find inserter", j.MKV{"name": in})
		}
	}
	for up, missing := range ups {
		if missing {
			return nil, errors.New("Couldn't find updater", j.MKV{"name": up})
		}
	}

	if err = ensureMatchingIDType(data.Inserters, data.Updaters); err != nil {
		return nil, err
	}

	var out bytes.Buffer
	if err = execTpl(&out, tpl, data); err != nil {
		return nil, errors.Wrap(err, "Failed executing template")
	}
	return imports.Process(filePath, out.Bytes(), nil)
}

func execTpl(out io.Writer, tpl string, data Data) error {
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

// ensureMatchingIDType returns an error if any of the inserters or updates have
// a different type for their ID.
func ensureMatchingIDType(inserters, updaters []Struct) error {
	var idType string
	for _, s := range append(inserters, updaters...) {
		if idType == "" {
			idType = s.IDType
		} else if idType != s.IDType {
			return ErrIDTypeMismatch
		}
	}
	return nil
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(col string) string {
	snake := matchFirstCap.ReplaceAllString(col, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}
