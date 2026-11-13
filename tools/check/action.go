// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"slices"
)

type expression struct {
	node    *node
	context context
}
type assignment struct {
	name       string
	expression expression
	choose     bool
}
type alternative struct {
	assignments []assignment
	guards      []expression
}

// Alternatives retain expressions until guards have been checked. Conjunction
// is a Cartesian product, not an imperative sequence of assignments.
func (e *evaluator) alternatives(n *node, c context, initial bool) ([]alternative, error) {
	if err := e.spend(); err != nil {
		return nil, err
	}
	if len(c.stack) > 128 {
		return nil, fmt.Errorf("action expansion depth exceeded")
	}
	switch {
	case n.op == "group":
		return e.alternatives(n.args[0], c, initial)
	case n.op == "name" || n.op == "call":
		_, bound := c.locals[n.text]
		if _, ok := e.module.defs[n.text]; ok && !bound {
			body, local, err := e.call(n, c)
			if err != nil {
				return nil, err
			}
			return e.alternatives(body, local, initial)
		}
	case n.op == "binary" && (n.text == "/\\" || n.text == "\\/"):
		left, err := e.alternatives(n.args[0], c, initial)
		if err != nil {
			return nil, err
		}
		right, err := e.alternatives(n.args[1], c, initial)
		if err != nil {
			return nil, err
		}
		if n.text == "\\/" {
			if len(left)+len(right) > e.maxValues {
				return nil, fmt.Errorf("action alternative bound exceeded")
			}
			return append(left, right...), nil
		}
		if len(right) > 0 && len(left) > e.maxValues/len(right) {
			return nil, fmt.Errorf("action alternative bound exceeded")
		}
		var out []alternative
		for _, a := range left {
			for _, b := range right {
				if len(a.assignments)+len(b.assignments)+len(a.guards)+len(b.guards) > e.maxValues {
					return nil, fmt.Errorf("action clause bound exceeded")
				}
				out = append(out, alternative{assignments: append(slices.Clone(a.assignments), b.assignments...), guards: append(slices.Clone(a.guards), b.guards...)})
			}
		}
		return out, nil
	case n.op == "binary" && (n.text == "=" || n.text == "\\in"):
		lhs := n.args[0]
		for lhs.op == "group" {
			lhs = lhs.args[0]
		}
		if slices.Contains(e.module.variables, lhs.text) && (initial && lhs.op == "name" || !initial && lhs.op == "prime") {
			if _, shadowed := c.locals[lhs.text]; shadowed {
				return nil, fmt.Errorf("assignment variable %s is shadowed", lhs.text)
			}
			level, err := e.module.expressionLevel(n.args[1], c)
			if err != nil {
				return nil, err
			}
			limit := 1
			if initial {
				limit = 0
			}
			if level > limit {
				return nil, fmt.Errorf("assignment to %s depends on an initial or primed variable", lhs.text)
			}
			return []alternative{{assignments: []assignment{{name: lhs.text, expression: expression{n.args[1], c}, choose: n.text == "\\in"}}}}, nil
		}
	case n.op == "UNCHANGED":
		if initial {
			return nil, fmt.Errorf("UNCHANGED is not an initial predicate")
		}
		names, err := e.module.variableList(n.args[0], nil)
		if err != nil {
			return nil, err
		}
		a := alternative{}
		for _, name := range names {
			if _, ok := c.locals[name]; ok {
				return nil, fmt.Errorf("UNCHANGED variable is shadowed")
			}
			a.assignments = append(a.assignments, assignment{name: name, expression: expression{&node{op: "name", text: name}, c}})
		}
		return []alternative{a}, nil
	case n.op == "\\E":
		level, err := e.module.expressionLevel(n.args[0], c)
		if err != nil {
			return nil, err
		}
		if initial && level > 0 {
			return nil, fmt.Errorf("initial quantifier domain depends on a state")
		}
		domain, err := e.eval(n.args[0], c)
		if err != nil {
			return nil, err
		}
		items, err := e.enumerate(domain)
		if err != nil {
			return nil, err
		}
		var out []alternative
		for _, v := range items {
			part, err := e.alternatives(n.args[1], c.bind(n.text, v), initial)
			if err != nil {
				return nil, err
			}
			if len(out)+len(part) > e.maxValues {
				return nil, fmt.Errorf("action alternative bound exceeded")
			}
			out = append(out, part...)
		}
		return out, nil
	case n.op == "if":
		level, err := e.module.expressionLevel(n.args[0], c)
		if err != nil {
			return nil, err
		}
		limit := 1
		if initial {
			limit = 0
		}
		if level > limit {
			return nil, fmt.Errorf("action IF guard depends on an unknown state")
		}
		v, err := e.eval(n.args[0], c)
		if err != nil {
			return nil, err
		}
		yes, err := v.truth()
		if err != nil {
			return nil, err
		}
		if yes {
			return e.alternatives(n.args[1], c, initial)
		}
		return e.alternatives(n.args[2], c, initial)
	}
	level, err := e.module.expressionLevel(n, c)
	if err != nil {
		return nil, err
	}
	limit := 2
	if initial {
		limit = 1
	}
	if level > limit {
		return nil, fmt.Errorf("temporal expression in a state relation")
	}
	return []alternative{{guards: []expression{{n, c}}}}, nil
}

func (e *evaluator) successors(n *node, before state, initial bool) ([]state, error) {
	alts, err := e.alternatives(n, context{before: before}, initial)
	if err != nil {
		return nil, err
	}
	unique := map[string]bool{}
	var out []state
	for _, a := range alts {
		var post []expression
		disabled := false
		var guardError error
		for _, g := range a.guards {
			level, err := e.module.expressionLevel(g.node, g.context)
			if err != nil {
				return nil, err
			}
			limit := 1
			if initial {
				limit = 0
			}
			if level > limit {
				post = append(post, g)
				continue
			}
			v, err := e.eval(g.node, g.context)
			if err == nil {
				var yes bool
				yes, err = v.truth()
				if err == nil && !yes {
					disabled = true
				}
			}
			if err != nil {
				guardError = err
			}
		}
		if disabled {
			continue
		}
		if guardError != nil {
			return nil, guardError
		}
		choices := []state{{}}
		assigned := map[string]bool{}
		for _, as := range a.assignments {
			if assigned[as.name] {
				return nil, fmt.Errorf("multiple assignments to %s are outside the constructive fragment", as.name)
			}
			assigned[as.name] = true
			v, err := e.eval(as.expression.node, as.expression.context)
			if err != nil {
				return nil, err
			}
			items := []value{v}
			if as.choose {
				items, err = e.enumerate(v)
				if err != nil {
					return nil, err
				}
			}
			if len(items) > 0 && len(choices) > e.maxValues/len(items) {
				return nil, fmt.Errorf("successor enumeration bound exceeded")
			}
			var next []state
			for _, s := range choices {
				for _, item := range items {
					item, err = e.canonical(item)
					if err != nil {
						return nil, err
					}
					t := make(state, len(s)+1)
					for name, v := range s {
						t[name] = v
					}
					t[as.name] = item
					next = append(next, t)
				}
			}
			choices = next
		}
		if len(choices) == 0 {
			continue
		}
		for _, name := range e.module.variables {
			if !assigned[name] {
				return nil, fmt.Errorf("unconstrained variable %s; assign it or use UNCHANGED", name)
			}
		}
		for _, s := range choices {
			keep := true
			for _, g := range post {
				c := g.context
				if initial {
					c.before = s
				} else {
					c.after = s
				}
				v, err := e.eval(g.node, c)
				if err != nil {
					return nil, err
				}
				yes, err := v.truth()
				if err != nil {
					return nil, err
				}
				if !yes {
					keep = false
					break
				}
			}
			if keep {
				key := s.key()
				if !unique[key] {
					if len(out) >= e.maxValues {
						return nil, fmt.Errorf("successor enumeration bound exceeded")
					}
					unique[key] = true
					out = append(out, s)
				}
			}
		}
	}
	return out, nil
}
