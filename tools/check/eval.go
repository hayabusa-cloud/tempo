// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math/big"
	"slices"
	"strings"
)

func (e *evaluator) call(n *node, c context) (*node, context, error) {
	d, ok := e.module.defs[n.text]
	if !ok {
		return nil, c, fmt.Errorf("line %d: unknown operator %s", n.line, n.text)
	}
	if len(d.params) != len(n.args) {
		return nil, c, fmt.Errorf("line %d: wrong arity for %s", n.line, n.text)
	}
	if slices.Contains(c.stack, n.text) || len(c.stack) >= 128 {
		return nil, c, fmt.Errorf("recursive operator %s is outside the supported fragment", n.text)
	}
	args := make([]value, len(n.args))
	for i, a := range n.args {
		v, err := e.eval(a, c)
		if err != nil {
			return nil, c, err
		}
		args[i] = v
	}
	// Operator parameters are lexical; caller LET/quantifier bindings do not leak.
	c.locals = map[string]value{}
	for i, name := range d.params {
		if _, ok := c.locals[name]; ok {
			return nil, c, fmt.Errorf("duplicate parameter %s", name)
		}
		c.locals[name] = args[i]
	}
	c.stack = append(slices.Clone(c.stack), n.text)
	return d.body, c, nil
}

func (e *evaluator) eval(n *node, c context) (value, error) {
	if err := e.spend(); err != nil {
		return value{}, err
	}
	v, err := e.evaluate(n, c)
	if err != nil {
		return value{}, err
	}
	if v.kind == "int" && v.n.BitLen() > 65536 {
		return value{}, fmt.Errorf("integer bit bound 65536 exceeded")
	}
	return v, nil
}

func (e *evaluator) evaluate(n *node, c context) (value, error) {
	if n == nil {
		return value{}, fmt.Errorf("missing expression")
	}
	get := func(i int) (value, error) { return e.eval(n.args[i], c) }
	switch n.op {
	case "group":
		return get(0)
	case "integer":
		z, ok := new(big.Int).SetString(n.text, 10)
		if !ok {
			return value{}, fmt.Errorf("invalid integer")
		}
		return value{kind: "int", n: z}, nil
	case "string":
		return value{kind: "string", s: n.text}, nil
	case "old":
		if c.old == nil {
			return value{}, fmt.Errorf("line %d: @ outside EXCEPT", n.line)
		}
		return *c.old, nil
	case "prime":
		v, ok := c.after[n.text]
		if !ok {
			return value{}, fmt.Errorf("line %d: unbound primed variable %s", n.line, n.text)
		}
		return v, nil
	case "name":
		if v, ok := c.locals[n.text]; ok {
			return v, nil
		}
		if v, ok := c.before[n.text]; ok {
			return v, nil
		}
		if v, ok := e.constants[n.text]; ok {
			return v, nil
		}
		switch n.text {
		case "TRUE":
			return boolean(true), nil
		case "FALSE":
			return boolean(false), nil
		case "BOOLEAN":
			return e.set([]value{boolean(false), boolean(true)})
		case "Nat":
			return value{kind: "nat"}, nil
		case "Int":
			return value{kind: "integers"}, nil
		}
		fallthrough
	case "call":
		body, local, err := e.call(n, c)
		if err != nil {
			return value{}, err
		}
		return e.eval(body, local)
	case "field":
		v, err := get(0)
		if err != nil {
			return value{}, err
		}
		return getField(v, n.text)
	case "record", "recordset":
		v := value{kind: n.op, fields: map[string]value{}}
		for i, name := range n.names {
			if _, ok := v.fields[name]; ok {
				return value{}, fmt.Errorf("duplicate field %s", name)
			}
			a, err := get(i)
			if err != nil {
				return value{}, err
			}
			v.fields[name] = a
		}
		return v, nil
	case "set", "tuple":
		items := make([]value, len(n.args))
		for i := range n.args {
			a, err := get(i)
			if err != nil {
				return value{}, err
			}
			items[i] = a
		}
		if n.op == "set" {
			return e.set(items)
		}
		return value{kind: "tuple", list: items}, nil
	case "except":
		base, err := get(0)
		if err != nil {
			return value{}, err
		}
		out := base
		for i, path := range n.names {
			for _, prior := range n.names[:i] {
				if path == prior || strings.HasPrefix(path, prior+".") || strings.HasPrefix(prior, path+".") {
					return value{}, fmt.Errorf("overlapping EXCEPT paths are unsupported")
				}
			}
			parts := strings.Split(path, ".")
			old, err := getPath(base, parts)
			if err != nil {
				return value{}, err
			}
			local := c
			local.old = &old
			a, err := e.eval(n.args[i+1], local)
			if err != nil {
				return value{}, err
			}
			out, err = replacePath(out, parts, a)
			if err != nil {
				return value{}, err
			}
		}
		return out, nil
	case "if":
		v, err := get(0)
		if err != nil {
			return value{}, err
		}
		yes, err := v.truth()
		if err != nil {
			return value{}, err
		}
		if yes {
			return get(1)
		}
		return get(2)
	case "case":
		var selected *node
		var otherwise *node
		for i := 0; i < len(n.args); i += 2 {
			if n.args[i].op == "other" {
				if otherwise != nil || i != len(n.args)-2 {
					return value{}, fmt.Errorf("OTHER must occur once at the end of CASE")
				}
				otherwise = n.args[i+1]
				continue
			}
			v, err := e.eval(n.args[i], c)
			if err != nil {
				return value{}, err
			}
			yes, err := v.truth()
			if err != nil {
				return value{}, err
			}
			if yes {
				if selected != nil {
					return value{}, fmt.Errorf("ambiguous CASE has multiple true guards")
				}
				selected = n.args[i+1]
			}
		}
		if selected == nil {
			selected = otherwise
		}
		if selected == nil {
			return value{}, fmt.Errorf("CASE has no selected branch")
		}
		return e.eval(selected, c)
	case "let":
		local := c
		seen := map[string]bool{}
		for i, name := range n.names {
			if seen[name] {
				return value{}, fmt.Errorf("duplicate LET binding %s", name)
			}
			seen[name] = true
			v, err := e.eval(n.args[i], local)
			if err != nil {
				return value{}, err
			}
			local = local.bind(name, v)
		}
		return e.eval(n.args[len(n.args)-1], local)
	case "\\E", "\\A":
		domain, err := get(0)
		if err != nil {
			return value{}, err
		}
		items, err := e.enumerate(domain)
		if err != nil {
			return value{}, err
		}
		for _, a := range items {
			v, err := e.eval(n.args[1], c.bind(n.text, a))
			if err != nil {
				return value{}, err
			}
			yes, err := v.truth()
			if err != nil {
				return value{}, err
			}
			if n.op == "\\E" && yes {
				return boolean(true), nil
			}
			if n.op == "\\A" && !yes {
				return boolean(false), nil
			}
		}
		return boolean(n.op == "\\A"), nil
	case "~":
		v, err := get(0)
		if err != nil {
			return value{}, err
		}
		b, err := v.truth()
		return boolean(!b), err
	case "-":
		v, err := get(0)
		if err != nil {
			return value{}, err
		}
		if v.kind != "int" {
			return value{}, fmt.Errorf("negation requires an integer")
		}
		return value{kind: "int", n: new(big.Int).Neg(v.n)}, nil
	case "binary":
		return e.binary(n, c)
	default:
		return value{}, fmt.Errorf("line %d: unsupported state expression %s", n.line, n.op)
	}
}

func (e *evaluator) binary(n *node, c context) (value, error) {
	a, err := e.eval(n.args[0], c)
	if err != nil {
		return value{}, err
	}
	if n.text == "/\\" || n.text == "\\/" || n.text == "=>" {
		b, err := a.truth()
		if err != nil {
			return value{}, err
		}
		if n.text == "/\\" && !b {
			return boolean(false), nil
		}
		if (n.text == "\\/" && b) || (n.text == "=>" && !b) {
			return boolean(true), nil
		}
	}
	b, err := e.eval(n.args[1], c)
	if err != nil {
		return value{}, err
	}
	switch n.text {
	case "/\\", "\\/", "=>":
		yes, err := b.truth()
		return boolean(yes), err
	case "<=>":
		x, err := a.truth()
		if err != nil {
			return value{}, err
		}
		y, err := b.truth()
		return boolean(x == y), err
	case "=", "#", "/=":
		same, err := e.equal(a, b)
		if n.text != "=" {
			same = !same
		}
		return boolean(same), err
	case "\\in", "\\notin":
		yes, err := e.member(a, b)
		if n.text == "\\notin" {
			yes = !yes
		}
		return boolean(yes), err
	}
	if a.kind != "int" || b.kind != "int" {
		return value{}, fmt.Errorf("operator %s requires integers", n.text)
	}
	switch n.text {
	case "+":
		return value{kind: "int", n: new(big.Int).Add(a.n, b.n)}, nil
	case "-":
		return value{kind: "int", n: new(big.Int).Sub(a.n, b.n)}, nil
	case "*":
		return value{kind: "int", n: new(big.Int).Mul(a.n, b.n)}, nil
	case "<":
		return boolean(a.n.Cmp(b.n) < 0), nil
	case ">":
		return boolean(a.n.Cmp(b.n) > 0), nil
	case "<=":
		return boolean(a.n.Cmp(b.n) <= 0), nil
	case ">=":
		return boolean(a.n.Cmp(b.n) >= 0), nil
	case "..":
		count := new(big.Int).Sub(b.n, a.n)
		count.Add(count, big.NewInt(1))
		if count.Sign() <= 0 {
			return e.set(nil)
		}
		if !count.IsInt64() || count.Int64() > int64(e.maxValues) {
			return value{}, fmt.Errorf("range enumeration bound exceeded")
		}
		items := make([]value, int(count.Int64()))
		for i := range items {
			items[i] = value{kind: "int", n: new(big.Int).Add(a.n, big.NewInt(int64(i)))}
		}
		return e.set(items)
	}
	return value{}, fmt.Errorf("unsupported operator %s", n.text)
}
