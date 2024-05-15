package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"text/template"
)

type mermaidDirection string

const (
	unknownDirection     mermaidDirection = ""
	topToBottomDirection mermaidDirection = "TB"
	leftToRightDirection mermaidDirection = "LR"
	rightToLeftDirection mermaidDirection = "RL"
	bottomToTopDirection mermaidDirection = "BT"
)

type mermaidTransition struct {
	From string
	To   string
}

type points []string
type transitions []mermaidTransition

type mermaidFormat struct {
	Direction      mermaidDirection
	StartingPoints points
	TerminalPoints points
	Transitions    transitions
}

func (t *points) add(point string) {
	// Check if point already exists
	if slices.Contains(*t, point) {
		return
	}

	*t = append(*t, point)
}

func (t *transitions) add(trans mermaidTransition) {
	// Check if transition already exists
	for _, val := range *t {
		if val.From == trans.From && val.To == trans.To {
			return
		}
	}

	*t = append(*t, trans)
}

func generateMermaidDiagram(pkgPath string) (string, error) {
	fs := token.NewFileSet()
	asts, err := parser.ParseDir(fs, pkgPath, nil, 0)

	if err != nil {
		return "", err
	}

	var diagram = &mermaidFormat{
		Direction: leftToRightDirection,
	}

	for _, node := range asts {
		shiftAlias := getShiftAlias(node)

		ast.Inspect(node, func(n ast.Node) bool {
			callExpr, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			return buildMermaidDiagram(callExpr, diagram, shiftAlias)
		})
	}

	return renderMermaidTpl(diagram)
}

func renderMermaidTpl(diagram *mermaidFormat) (string, error) {
	t, err := template.New("").Parse(mermaidTemplate)

	if err != nil {
		return "", err
	}

	buf := new(bytes.Buffer)

	err = t.Execute(buf, diagram)

	return buf.String(), err
}

func getShiftAlias(node *ast.Package) string {
	shiftAlias := "shift" // Default package name

	ast.Inspect(node, func(n ast.Node) bool {
		importSpec, ok := n.(*ast.ImportSpec)
		if !ok {
			return true
		}
		if importSpec.Path.Value == `"github.com/luno/shift"` {
			if importSpec.Name != nil {
				shiftAlias = importSpec.Name.Name
			}
			return false
		}
		return true
	})

	return shiftAlias
}

// buildMermaidDiagram captures information about .Insert and .Update calls.
func buildMermaidDiagram(expr *ast.CallExpr, diagram *mermaidFormat, shiftAlias string) bool {
	selectorExpr, ok := expr.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	// Check for the NewArcFSM at the beginning of the chain
	if isShiftCall(expr, "NewArcFSM", shiftAlias) {
		if selectorExpr.Sel.Name == "Insert" {
			if len(expr.Args) > 0 {
				firstArg := formatArg(expr.Args[0])
				diagram.StartingPoints.add(firstArg)
			}
		}

		if selectorExpr.Sel.Name == "Update" {
			if len(expr.Args) >= 2 {
				firstArg := formatArg(expr.Args[0])
				secondArg := formatArg(expr.Args[1])
				diagram.Transitions.add(mermaidTransition{From: firstArg, To: secondArg})
			}
		}
	}

	// Check for the NewFSM at the beginning of the chain
	if isShiftCall(expr, "NewFSM", shiftAlias) {
		if selectorExpr.Sel.Name == "Insert" {
			if len(expr.Args) == 2 {
				firstArg := formatArg(expr.Args[0])
				diagram.StartingPoints.add(firstArg)
			} else if len(expr.Args) > 2 {
				firstArg := formatArg(expr.Args[0])
				diagram.StartingPoints.add(firstArg)

				for _, arg := range expr.Args[2:] {
					diagram.Transitions.add(mermaidTransition{From: firstArg, To: formatArg(arg)})
				}
			}
		}

		if selectorExpr.Sel.Name == "Update" {
			if len(expr.Args) == 2 {
				diagram.TerminalPoints.add(formatArg(expr.Args[0]))
			} else if len(expr.Args) > 2 {
				firstArg := formatArg(expr.Args[0])

				for _, arg := range expr.Args[2:] {
					diagram.Transitions.add(mermaidTransition{From: firstArg, To: formatArg(arg)})
				}
			}
		}
	}

	return true
}

// isShiftCall checks if the expression is a chain of method calls starting with the shift package alias.
func isShiftCall(expr *ast.CallExpr, methodCall, shiftAlias string) bool {
	for {
		selectorExpr, ok := expr.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if selectorExpr.Sel.Name == methodCall {
			ident, ok := selectorExpr.X.(*ast.Ident)
			if !ok {
				return false
			}
			if ident.Name == shiftAlias {
				return true
			}
		}
		if callExpr, ok := selectorExpr.X.(*ast.CallExpr); ok {
			expr = callExpr
			continue
		}
		return false
	}
}

func formatArg(arg ast.Expr) string {
	switch a := arg.(type) {
	case *ast.Ident:
		return a.Name
	case *ast.SelectorExpr:
		if _, ok := a.X.(*ast.Ident); ok {
			return a.Sel.Name
		}
	}

	return fmt.Sprintf("%s", arg)
}
