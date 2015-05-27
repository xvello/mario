package parser

import (
	"regexp"

	"github.com/aymerick/raymond/ast"
)

//
// The logic was shamelessly borrowed from:
//   https://github.com/wycats/handlebars.js/blob/master/lib/handlebars/compiler/whitespace-control.js
//

type WhitespaceVisitor struct {
	isRootSeen bool
}

var (
	rTrimLeft         = regexp.MustCompile(`^[ \t]*\r?\n?`)
	rTrimLeftMultiple = regexp.MustCompile(`^\s+`)

	rTrimRight         = regexp.MustCompile(`[ \t]+$`)
	rTrimRightMultiple = regexp.MustCompile(`\s+$`)

	rPrevWhitespace      = regexp.MustCompile(`\r?\n\s*?$`)
	rPrevWhitespaceStart = regexp.MustCompile(`(^|\r?\n)\s*?$`)

	rNextWhitespace    = regexp.MustCompile(`^\s*?\r?\n`)
	rNextWhitespaceEnd = regexp.MustCompile(`^\s*?(\r?\n|$)`)
)

func NewWhitespaceVisitor() *WhitespaceVisitor {
	return &WhitespaceVisitor{}
}

func ProcessWhitespaces(node ast.Node) {
	node.Accept(NewWhitespaceVisitor())
}

func omitRightFirst(body []ast.Node, multiple bool) {
	omitRight(body, -1, multiple)
}

func omitRight(body []ast.Node, i int, multiple bool) {
	if i+1 >= len(body) {
		return
	}

	current := body[i+1]

	node, ok := current.(*ast.ContentStatement)
	if !ok {
		return
	}

	if !multiple && node.RightStripped {
		return
	}

	original := node.Value

	r := rTrimLeft
	if multiple {
		r = rTrimLeftMultiple
	}

	node.Value = r.ReplaceAllString(node.Value, "")

	node.RightStripped = (original != node.Value)
}

func omitLeftLast(body []ast.Node, multiple bool) {
	omitLeft(body, len(body), multiple)
}

func omitLeft(body []ast.Node, i int, multiple bool) bool {
	if i-1 < 0 {
		return false
	}

	current := body[i-1]

	node, ok := current.(*ast.ContentStatement)
	if !ok {
		return false
	}

	if !multiple && node.LeftStripped {
		return false
	}

	original := node.Value

	r := rTrimRight
	if multiple {
		r = rTrimRightMultiple
	}

	node.Value = r.ReplaceAllString(node.Value, "")

	node.LeftStripped = (original != node.Value)

	return node.LeftStripped
}

func isPrevWhitespace(body []ast.Node) bool {
	return isPrevWhitespaceProgram(body, len(body), false)
}

func isPrevWhitespaceProgram(body []ast.Node, i int, isRoot bool) bool {
	if i < 1 {
		return isRoot
	}

	prev := body[i-1]

	if node, ok := prev.(*ast.ContentStatement); ok {
		r := rPrevWhitespaceStart
		if (i > 1) || !isRoot {
			r = rPrevWhitespace
		}

		return r.MatchString(node.Value)
	}

	return false
}

func isNextWhitespace(body []ast.Node) bool {
	return isNextWhitespaceProgram(body, -1, false)
}

func isNextWhitespaceProgram(body []ast.Node, i int, isRoot bool) bool {
	if i+1 >= len(body) {
		return isRoot
	}

	next := body[i+1]

	if node, ok := next.(*ast.ContentStatement); ok {
		r := rNextWhitespaceEnd
		if (i+2 >= len(body)) || !isRoot {
			r = rNextWhitespace
		}

		return r.MatchString(node.Value)
	}

	return false
}

//
// Visitor interface
//

func (v *WhitespaceVisitor) VisitProgram(program *ast.Program) interface{} {
	isRoot := !v.isRootSeen
	v.isRootSeen = true

	body := program.Body
	for i, current := range body {
		strip, _ := current.Accept(v).(*ast.Strip)
		if strip == nil {
			continue
		}

		_isPrevWhitespace := isPrevWhitespaceProgram(body, i, isRoot)
		_isNextWhitespace := isNextWhitespaceProgram(body, i, isRoot)

		openStandalone := strip.OpenStandalone && _isPrevWhitespace
		closeStandalone := strip.CloseStandalone && _isNextWhitespace
		inlineStandalone := strip.InlineStandalone && _isPrevWhitespace && _isNextWhitespace

		if strip.Close {
			omitRight(body, i, true)
		}

		if strip.Open && (i > 0) {
			omitLeft(body, i, true)
		}

		if inlineStandalone {
			omitRight(body, i, false)

			if omitLeft(body, i, false) {
				// If we are on a standalone node, save the indent info for partials
				if _, ok := current.(*ast.PartialStatement); ok {
					// Pull out the whitespace from the final line
					// @todo partial.Indent = (/([ \t]+$)/).exec(body[i - 1].original)[1];
				}
			}
		}

		if b, ok := current.(*ast.BlockStatement); ok {
			if openStandalone {
				prog := b.Program
				if prog == nil {
					prog = b.Inverse
				}

				omitRightFirst(prog.Body, false)

				// Strip out the previous content node if it's whitespace only
				omitLeft(body, i, false)
			}

			if closeStandalone {
				prog := b.Inverse
				if prog == nil {
					prog = b.Program
				}

				// Always strip the next node
				omitRight(body, i, false)

				omitLeftLast(prog.Body, false)
			}

		}
	}

	return nil
}

func (v *WhitespaceVisitor) VisitBlock(block *ast.BlockStatement) interface{} {
	if block.Program != nil {
		block.Program.Accept(v)
	}

	if block.Inverse != nil {
		block.Inverse.Accept(v)
	}

	program := block.Program
	inverse := block.Inverse

	if program == nil {
		program = inverse
		inverse = nil
	}

	firstInverse := inverse
	lastInverse := inverse

	if (inverse != nil) && inverse.Chained {
		b, _ := inverse.Body[0].(*ast.BlockStatement)
		firstInverse = b.Program

		for lastInverse.Chained {
			b, _ := lastInverse.Body[len(lastInverse.Body)-1].(*ast.BlockStatement)
			lastInverse = b.Program
		}
	}

	closeProg := firstInverse
	if closeProg == nil {
		closeProg = program
	}

	strip := &ast.Strip{
		Open:  (block.OpenStrip != nil) && block.OpenStrip.Open,
		Close: (block.CloseStrip != nil) && block.CloseStrip.Close,

		OpenStandalone:  isNextWhitespace(program.Body),
		CloseStandalone: isPrevWhitespace(closeProg.Body),
	}

	if (program != nil) && (block.OpenStrip != nil) && block.OpenStrip.Close {
		omitRightFirst(program.Body, true)
	}

	if inverse != nil {
		if block.InverseStrip != nil {
			inverseStrip := block.InverseStrip

			if inverseStrip.Open {
				omitLeftLast(program.Body, true)
			}

			if inverseStrip.Close {
				omitRightFirst(firstInverse.Body, true)
			}
		}

		if (block.CloseStrip != nil) && block.CloseStrip.Open {
			omitLeftLast(lastInverse.Body, true)
		}

		if isPrevWhitespace(program.Body) && isNextWhitespace(firstInverse.Body) {
			omitLeftLast(program.Body, false)
			omitRightFirst(firstInverse.Body, false)
		}
	} else if (block.CloseStrip != nil) && block.CloseStrip.Open {
		omitLeftLast(program.Body, true)
	}

	return strip
}

func (v *WhitespaceVisitor) VisitMustache(mustache *ast.MustacheStatement) interface{} {
	return mustache.Strip
}

func _inlineStandalone(strip *ast.Strip) interface{} {
	return &ast.Strip{
		Open:             strip.Open,
		Close:            strip.Close,
		InlineStandalone: true,
	}
}

func (v *WhitespaceVisitor) VisitPartial(node *ast.PartialStatement) interface{} {
	strip := node.Strip
	if strip == nil {
		strip = &ast.Strip{}
	}

	return _inlineStandalone(strip)
}

func (v *WhitespaceVisitor) VisitComment(node *ast.CommentStatement) interface{} {
	strip := node.Strip
	if strip == nil {
		strip = &ast.Strip{}
	}

	return _inlineStandalone(strip)
}

// NOOP
func (v *WhitespaceVisitor) VisitContent(node *ast.ContentStatement) interface{}    { return nil }
func (v *WhitespaceVisitor) VisitExpression(node *ast.Expression) interface{}       { return nil }
func (v *WhitespaceVisitor) VisitSubExpression(node *ast.SubExpression) interface{} { return nil }
func (v *WhitespaceVisitor) VisitPath(node *ast.PathExpression) interface{}         { return nil }
func (v *WhitespaceVisitor) VisitString(node *ast.StringLiteral) interface{}        { return nil }
func (v *WhitespaceVisitor) VisitBoolean(node *ast.BooleanLiteral) interface{}      { return nil }
func (v *WhitespaceVisitor) VisitNumber(node *ast.NumberLiteral) interface{}        { return nil }
func (v *WhitespaceVisitor) VisitHash(node *ast.Hash) interface{}                   { return nil }
func (v *WhitespaceVisitor) VisitHashPair(node *ast.HashPair) interface{}           { return nil }