// Copyright 2012-2014, Rolf Veen and contributors.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ogdl

import "fmt"

// Eval takes a parsed expression and evaluates it
// in the context of the current graph
//
// Example:
//
//     g := ogdl.NewGraph("a").Add("2")
//     e := ogdl.NewExpression("a+2")
//     println(g.Eval(e))
//
func (g *Graph) Eval(e *Graph) interface{} {

	switch (e.String()) {
	    case TYPE_PATH: return g.EvalPath(e)
	    case TYPE_EXPRESSION: return g.EvalExpression(e)
	}

	if e.Len() != 0 {
		return e
	}

	// Return constant in its normalizad native form
	// either: int64, float64, string, bool or []byte
	return e.Scalar()
}

// Eval takes a parsed expression and evaluates it
// in the context of the current graph, and converting the result to a boolean.
func (g *Graph) EvalBool(e *Graph) bool {
	b, _ := _boolf(g.Eval(e))
	return b
}

// EvalPath traverses g following a path p. The path needs to be previously converted
// to a Graph with NewPath().
//
// This function is similar to ogdl.Get, but for complex paths.
func (g *Graph) EvalPath(p *Graph) interface{} {

	if p.Len() == 0 {
		return nil
	}

	// Normalize the context graph, so that the root is
	// always transparent.

	var node *Graph

	if !g.IsNil() {
		node = NilGraph()
		node.Add(g)
	} else {
		node = g
	}

	iknow := false

	for i := 0; i < len(p.Out); i++ {
		n := p.Out[i]

		// For each path element, look at its type
		// token, index, selector, arglist
		s := n.String()

		iknow = false

		switch s {

		case TYPE_INDEX:
			// must evaluate to an integer
			if n.Len() == 0 {
				return "empty []"
			}
			itf := g.EvalExpression(n.Out[0])
			ix, ok := _int64(itf)
			if !ok || ix < 0 || int(ix) >= node.Len() {
				return "[] does not evaluate to a valid integer"
			}
			node = node.GetAt(int(ix))

		case TYPE_SELECTOR:
			return "{} not supported yet"

		case "_len":
			return node.Len()

		case TYPE_GROUP:
			// The following format is supported: ( expression )
			// The expression is evaluated and used as path element
			itf := g.EvalExpression(n.Out[0])
			str := _string(itf)
			if len(str) == 0 {
				return nil //"(expr) does not evaluate to a string in path: "+p.Text()
			}
			s = str
			fallthrough
		default:
			nn := node.Node(s)

			if nn == nil {
				// It may have a !type
				itf, _ := node.Function(p, i, g)
				return itf
			}

			iknow = true

			node = nn
		}
	}

	if iknow && node != nil {
		if node.Len() == 1 && node.Out[0].Len() == 0 {
			return node.Out[0].This
		}

		node2 := NilGraph()
		node2.Out = node.Out
		return node2
	}

	return node
}


//
// g can have native types (other things than strings), but
// p only []byte or string
//
func (g *Graph) EvalExpression(p *Graph) interface{} {

	// Return nil and empty strings as is
	if p.This == nil {
		return nil
	}

	s := p.String()

	if len(s) == 0 {
		return ""
	}

	// first check if it is a number because it can have an operatorChar
	// in front: the minus sign
	if isNumber(s) {
		return p.Number()
	}

	switch s {
	case "!":
		// Unary expression !expr
		return !g.EvalBool(p.Out[0])
	case TYPE_EXPRESSION:
		return g.EvalExpression(p.GetAt(0))
	case TYPE_PATH:
		return g.EvalPath(p)
	case TYPE_GROUP:
		// expression list
		r := NewGraph(TYPE_GROUP)
		for _, expr := range p.Out {
			r.Add(g.EvalExpression(expr))
		}
		return r
	}

	c := int(s[0])

	if IsOperatorChar(c) {
		return g.evalBinary(p)
	}

	if c == '"' || c == '\'' {
		return s
	}

	if IsLetter(c) {
		if s == "false" {
			return false
		}
		if s == "true" {
			return true
		}
		return s
	}

	return p
}

func (g *Graph) evalBinary(p *Graph) interface{} {
	// p.String() is the operator

	n1 := p.Out[0]
	i2 := g.EvalExpression(p.Out[1])

	switch p.String() {

	case "+":
		return calc(g.EvalExpression(n1), i2, '+')
	case "-":
		return calc(g.EvalExpression(n1), i2, '-')
	case "*":
		return calc(g.EvalExpression(n1), i2, '*')
	case "/":
		return calc(g.EvalExpression(n1), i2, '/')
	case "%":
		return calc(g.EvalExpression(n1), i2, '%')

	case "=":
		return g.assign(n1, i2, '=')
	case "+=":
		return g.assign(n1, i2, '+')
	case "-=":
		return g.assign(n1, i2, '-')
	case "*=":
		return g.assign(n1, i2, '*')
	case "/=":
		return g.assign(n1, i2, '/')
	case "%=":
		return g.assign(n1, i2, '%')

	case "==":
		return compare(g.EvalExpression(n1), i2, '=')
	case ">=":
		return compare(g.EvalExpression(n1), i2, '+')
	case "<=":
		return compare(g.EvalExpression(n1), i2, '-')
	case "!=":
		return compare(g.EvalExpression(n1), i2, '!')
	case ">":
		return compare(g.EvalExpression(n1), i2, '>')
	case "<":
		return compare(g.EvalExpression(n1), i2, '<')

	case "&&":
		return logic(g.EvalExpression(n1), i2, '&')
	case "||":
		return logic(g.EvalExpression(n1), i2, '|')

	}

	return nil
}

// int* | float* | string
// first element determines type
func compare(v1, v2 interface{}, op int) bool {
	fmt.Printf("compare [%v] [%v]\n", v1, v2)
	i1, ok := _int64(v1)

	if ok {
		i2, ok := _int64f(v2)
		if !ok {
			return false
		}

		switch op {
		case '=':
			return i1 == i2
		case '+':
			return i1 >= i2
		case '-':
			return i1 <= i2
		case '>':
			return i1 > i2
		case '<':
			return i1 < i2
		case '!':
			return i1 != i2
		}
		return false
	}

	f1, ok := _float64(v1)
	if ok {
		f2, ok := _float64f(v2)
		if !ok {
			return false
		}
		switch op {
		case '=':
			return f1 == f2
		case '+':
			return f1 >= f2
		case '-':
			return f1 <= f2
		case '>':
			return f1 > f2
		case '<':
			return f1 < f2
		case '!':
			return f1 != f2
		}
		return false
	}

	s1 := _string(v1)
	s2 := _string(v2)

	switch op {
	case '=':
		return s1 == s2
	case '!':
		return s1 != s2
	}
	return false
}

func logic(i1, i2 interface{}, op int) bool {

	b1, ok1 := _boolf(i1)
	b2, ok2 := _boolf(i2)

	if !ok1 || !ok2 {
		return false
	}

	switch op {
	case '&':
		return b1 && b2
	case '|':
		return b1 || b2
	}

	return false
}

// assign modifies the context graph
func (g *Graph) assign(p *Graph, v interface{}, op int) interface{} {

	if op == '=' {
		return g.set(p, v)
	}

	// if p doesn't exist, just set it to the value given
	left := g.get(p)
	if left == nil {
		return g.set(p, calc(left.This, v, op))
	}

	switch op {
	case '+':
		return g.set(p, v)
	case '-':
		return g.set(p, calc(0, v, '-'))
	case '*':
		return g.set(p, 0)
	case '/':
		return g.set(p, "infinity")
	case '%':
		return g.set(p, "undefined")
	}

	return nil
}

// calc: int64 | float64 | string
func calc(v1, v2 interface{}, op int) interface{} {

	i1, ok := v1.(int64)
	i2, ok2 := v2.(int64)

	var ok3, ok4 bool
	var i3, i4 float64

	if !ok {
		i3, ok3 = v1.(float64)
	}
	if !ok2 {
		i4, ok4 = v2.(float64)
	}

	if ok && ok2 {
		switch op {
		case '+':
			return i1 + i2
		case '-':
			return i1 - i2
		case '*':
			return i1 * i2
		case '/':
			return i1 / i2
		case '%':
			return i1 % i2
		}
	}
	if ok3 && ok4 {
		switch op {
		case '+':
			return i3 + i4
		case '-':
			return i3 - i4
		case '*':
			return i3 * i4
		case '/':
			return i3 / i4
		case '%':
			return int(i3) % int(i4)
		}
	}
	if ok && ok4 {
		i3 = float64(i1)
		switch op {
		case '+':
			return i3 + i4
		case '-':
			return i3 - i4
		case '*':
			return i3 * i4
		case '/':
			return i3 / i4
		case '%':
			return i1 % int64(i4)
		}
	}
	if ok3 && ok2 {
		i4 = float64(i2)
		switch op {
		case '+':
			return i3 + i4
		case '-':
			return i3 - i4
		case '*':
			return i3 * i4
		case '/':
			return i3 / i4
		case '%':
			return int64(i3) % i2
		}
	}

	if op != '+' {
		return nil
	}

	return _string(v1) + _string(v2)
}