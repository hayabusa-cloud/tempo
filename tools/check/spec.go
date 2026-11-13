// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"slices"
	"strings"
)

type propertyKind uint8

const (
	invariantProperty propertyKind = iota
	eventualProperty
	responseProperty
	recurrenceProperty
)

type property struct {
	name               string
	predicate, trigger *node
	kind               propertyKind
}
type specification struct {
	initial, next *node
	fairness      []*node
	checks        []property
}

func conjunction(a, b *node) *node {
	if a == nil {
		return b
	}
	return &node{op: "binary", text: "/\\", args: []*node{a, b}}
}

func (m *module) expand(n *node, stack []string) (*node, error) {
	for {
		if n.op == "group" {
			n = n.args[0]
			continue
		}
		if n.op != "name" {
			return n, nil
		}
		d, ok := m.defs[n.text]
		if !ok {
			return n, nil
		}
		if len(d.params) != 0 || slices.Contains(stack, n.text) || len(stack) >= 128 {
			return nil, fmt.Errorf("invalid or recursive specification alias %s", n.text)
		}
		stack = append(stack, n.text)
		n = d.body
	}
}

func (m *module) specification(c *configuration) (*specification, error) {
	s := &specification{}
	root := &node{op: "name", text: c.spec}
	if _, err := m.level(root, nil, nil); err != nil {
		return nil, err
	}
	var collect func(*node) error
	collect = func(n *node) error {
		l, err := m.level(n, nil, nil)
		if err != nil {
			return err
		}
		if l <= 1 {
			s.initial = conjunction(s.initial, n)
			return nil
		}
		n, err = m.expand(n, nil)
		if err != nil {
			return err
		}
		switch {
		case n.op == "binary" && n.text == "/\\":
			if err := collect(n.args[0]); err != nil {
				return err
			}
			return collect(n.args[1])
		case n.op == "[]":
			a, err := m.expand(n.args[0], nil)
			if err != nil {
				return err
			}
			if a.op != "actionbox" {
				return fmt.Errorf("specification requires [][Next]_vars")
			}
			if s.next != nil {
				return fmt.Errorf("specification has more than one next relation")
			}
			if err := m.fullSubscript(a.text); err != nil {
				return err
			}
			s.next = a.args[0]
			return nil
		case n.op == "call" && strings.HasPrefix(n.text, "WF_"):
			if err := m.fullSubscript(strings.TrimPrefix(n.text, "WF_")); err != nil {
				return err
			}
			s.fairness = append(s.fairness, n.args[0])
			return nil
		default:
			return fmt.Errorf("unsupported specification; expected Init /\\ [][Next]_vars and optional WF_vars actions")
		}
	}
	if err := collect(root); err != nil {
		return nil, err
	}
	if s.initial == nil || s.next == nil {
		return nil, fmt.Errorf("both initial and next predicates are required")
	}
	for _, name := range c.invariants {
		n := &node{op: "name", text: name}
		l, err := m.level(n, nil, nil)
		if err != nil {
			return nil, err
		}
		if l > 1 {
			return nil, fmt.Errorf("invariant %s must be a state predicate", name)
		}
		s.checks = append(s.checks, property{name: name, predicate: n})
	}
	for _, name := range c.properties {
		n := &node{op: "name", text: name}
		if _, err := m.level(n, nil, nil); err != nil {
			return nil, err
		}
		n, err := m.expand(n, nil)
		if err != nil {
			return nil, err
		}
		p, err := m.temporalProperty(name, n)
		if err != nil {
			return nil, err
		}
		s.checks = append(s.checks, p)
	}
	return s, nil
}

func (m *module) temporalProperty(name string, n *node) (property, error) {
	p := property{name: name}
	unsupported := func() (property, error) {
		return p, fmt.Errorf("property %s must be []P, <>P, []<>P, or [](P => <>Q) for state predicates", name)
	}
	if n.op != "<>" && n.op != "[]" {
		return unsupported()
	}
	inner, err := m.expand(n.args[0], nil)
	if err != nil {
		return p, err
	}
	level, err := m.level(inner, nil, nil)
	if err != nil {
		return p, err
	}
	if level <= 1 {
		p.predicate = inner
		if n.op == "<>" {
			p.kind = eventualProperty
		}
		return p, nil
	}
	if n.op != "[]" {
		return unsupported()
	}
	switch {
	case inner.op == "<>":
		p.kind, p.predicate = recurrenceProperty, inner.args[0]
	case inner.op == "binary" && inner.text == "=>":
		goal, err := m.expand(inner.args[1], nil)
		if err != nil {
			return p, err
		}
		if goal.op != "<>" {
			return unsupported()
		}
		p.kind, p.trigger, p.predicate = responseProperty, inner.args[0], goal.args[0]
	default:
		return unsupported()
	}
	for _, predicate := range []*node{p.predicate, p.trigger} {
		if predicate == nil {
			continue
		}
		level, err := m.level(predicate, nil, nil)
		if err != nil {
			return p, err
		}
		if level > 1 {
			return unsupported()
		}
	}
	return p, nil
}

func configure(m *module, c *configuration, b bounds) (*evaluator, error) {
	e := &evaluator{module: m, constants: map[string]value{}, maxValues: b.values, maxSteps: b.steps}
	// Configuration values are closed expressions. They cannot reference model
	// operators, state, or other parameter substitutions.
	closed := &evaluator{module: &module{naturals: true, integers: true}, maxValues: b.values, maxSteps: b.steps}
	for name := range c.constants {
		if !slices.Contains(m.constants, name) {
			return nil, fmt.Errorf("undeclared configuration constant %s", name)
		}
	}
	for _, name := range m.constants {
		n, ok := c.constants[name]
		if !ok {
			return nil, fmt.Errorf("missing constant %s", name)
		}
		if err := closed.module.bindings(n, map[string]bool{}); err != nil {
			return nil, err
		}
		if _, err := closed.module.level(n, nil, nil); err != nil {
			return nil, err
		}
		v, err := closed.eval(n, context{})
		if err != nil {
			return nil, err
		}
		v, err = e.canonical(v)
		if err != nil {
			return nil, err
		}
		e.constants[name] = v
	}
	for _, a := range m.assumes {
		l, err := m.level(a, nil, nil)
		if err != nil {
			return nil, err
		}
		if l != 0 {
			return nil, fmt.Errorf("ASSUME must be a constant expression")
		}
		v, err := e.eval(a, context{})
		if err != nil {
			return nil, err
		}
		yes, err := v.truth()
		if err != nil {
			return nil, err
		}
		if !yes {
			return nil, fmt.Errorf("ASSUME is false for the supplied constants")
		}
	}
	e.steps += closed.steps
	return e, nil
}
