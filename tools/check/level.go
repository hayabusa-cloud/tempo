// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Binding legality uses the declaration-time context, before later globals
// exist. Rechecking it when an operator is expanded would capture valid locals.
func (m *module) bindName(name string, bound map[string]bool) error {
	_, defined := m.defs[name]
	if bound[name] || defined || slices.Contains(m.constants, name) || slices.Contains(m.variables, name) || name == "Nat" || name == "Int" {
		return fmt.Errorf("binding %s shadows an existing name", name)
	}
	bound[name] = true
	return nil
}

func (m *module) bindings(n *node, bound map[string]bool) error {
	switch n.op {
	case "let":
		local := maps.Clone(bound)
		for i, name := range n.names {
			if err := m.bindings(n.args[i], local); err != nil {
				return err
			}
			if err := m.bindName(name, local); err != nil {
				return err
			}
		}
		return m.bindings(n.args[len(n.args)-1], local)
	case "\\E", "\\A":
		if err := m.bindings(n.args[0], bound); err != nil {
			return err
		}
		local := maps.Clone(bound)
		if err := m.bindName(n.text, local); err != nil {
			return err
		}
		return m.bindings(n.args[1], local)
	}
	for _, a := range n.args {
		if err := m.bindings(a, bound); err != nil {
			return err
		}
	}
	return nil
}

// Levels are constant, state, action, and temporal, respectively.
func (m *module) level(n *node, bound map[string]int, stack []string) (int, error) {
	if n == nil {
		return 0, fmt.Errorf("missing expression")
	}
	lookup := func() (int, error) {
		if v, ok := bound[n.text]; ok {
			return v, nil
		}
		if slices.Contains(m.variables, n.text) {
			return 1, nil
		}
		if slices.Contains(m.constants, n.text) {
			return 0, nil
		}
		switch n.text {
		case "TRUE", "FALSE", "BOOLEAN":
			return 0, nil
		case "Nat":
			if !m.naturals {
				return 0, fmt.Errorf("Nat requires EXTENDS Naturals or Integers")
			}
			return 0, nil
		case "Int":
			if !m.integers {
				return 0, fmt.Errorf("Int requires EXTENDS Integers")
			}
			return 0, nil
		}
		return -1, nil
	}
	if n.op == "name" {
		if l, err := lookup(); l >= 0 || err != nil {
			return l, err
		}
	}
	if n.op == "name" || n.op == "call" {
		if _, ok := bound[n.text]; ok {
			return 0, fmt.Errorf("value binding %s cannot be applied as an operator", n.text)
		}
		if strings.HasPrefix(n.text, "WF_") {
			if len(n.args) != 1 {
				return 0, fmt.Errorf("WF requires one action")
			}
			if _, err := m.variableList(&node{op: "name", text: strings.TrimPrefix(n.text, "WF_")}, nil); err != nil {
				return 0, err
			}
			l, err := m.level(n.args[0], bound, stack)
			if err != nil {
				return 0, err
			}
			if l > 2 {
				return 0, fmt.Errorf("temporal fairness action")
			}
			return 3, nil
		}
		d, ok := m.defs[n.text]
		if !ok {
			return 0, fmt.Errorf("line %d: unknown or unsupported operator %s", n.line, n.text)
		}
		if len(d.params) != len(n.args) {
			return 0, fmt.Errorf("wrong arity for %s", n.text)
		}
		if slices.Contains(stack, n.text) || len(stack) >= 128 {
			return 0, fmt.Errorf("recursive operator %s is unsupported", n.text)
		}
		local := map[string]int{}
		key := n.text + ":"
		for i, name := range d.params {
			l, err := m.level(n.args[i], bound, stack)
			if err != nil {
				return 0, err
			}
			if l > 1 {
				return 0, fmt.Errorf("operator arguments must be values, not actions")
			}
			local[name] = l
			key += fmt.Sprint(l)
		}
		if l, ok := m.levels[key]; ok {
			return l, nil
		}
		l, err := m.level(d.body, local, append(slices.Clone(stack), n.text))
		if err == nil {
			if m.levels == nil {
				m.levels = map[string]int{}
			}
			m.levels[key] = l
		}
		return l, err
	}
	switch n.op {
	case "-":
		if !m.integers {
			return 0, fmt.Errorf("unary minus requires EXTENDS Integers")
		}
	case "binary":
		switch n.text {
		case "+", "-", "*", "<", ">", "<=", ">=", "..":
			if !m.naturals {
				return 0, fmt.Errorf("arithmetic requires EXTENDS Naturals or Integers")
			}
		}
	case "prime":
		if _, ok := bound[n.text]; ok || !slices.Contains(m.variables, n.text) {
			return 0, fmt.Errorf("only an unshadowed declared variable may be primed")
		}
		return 2, nil
	case "UNCHANGED":
		if err := unshadowed(n.args[0], bound); err != nil {
			return 0, err
		}
		if _, err := m.variableList(n.args[0], nil); err != nil {
			return 0, err
		}
		return 2, nil
	case "let":
		local := maps.Clone(bound)
		if local == nil {
			local = map[string]int{}
		}
		seen := map[string]bool{}
		highest := 0
		for i, name := range n.names {
			if seen[name] {
				return 0, fmt.Errorf("duplicate LET binding %s", name)
			}
			seen[name] = true
			l, err := m.level(n.args[i], local, stack)
			if err != nil {
				return 0, err
			}
			if l > 1 {
				return 0, fmt.Errorf("LET bindings must be values")
			}
			local[name] = l
			highest = max(highest, l)
		}
		l, err := m.level(n.args[len(n.args)-1], local, stack)
		return max(highest, l), err
	case "\\E", "\\A":
		d, err := m.level(n.args[0], bound, stack)
		if err != nil {
			return 0, err
		}
		if d > 1 {
			return 0, fmt.Errorf("quantifier domains must be state expressions")
		}
		local := maps.Clone(bound)
		if local == nil {
			local = map[string]int{}
		}
		local[n.text] = d
		l, err := m.level(n.args[1], local, stack)
		return max(d, l), err
	}
	highest := 0
	for _, a := range n.args {
		l, err := m.level(a, bound, stack)
		if err != nil {
			return 0, err
		}
		highest = max(highest, l)
	}
	switch n.op {
	case "[]", "<>":
		if highest == 2 {
			a, err := m.expand(n.args[0], nil)
			if err != nil {
				return 0, err
			}
			if n.op != "[]" || a.op != "actionbox" {
				return 0, fmt.Errorf("temporal operators require state predicates or [][A]_vars")
			}
		}
		return 3, nil
	case "actionbox":
		if highest > 2 {
			return 0, fmt.Errorf("temporal expression inside an action")
		}
		return 2, nil
	}
	return highest, nil
}

func unshadowed(n *node, bound map[string]int) error {
	if n.op == "name" {
		if _, ok := bound[n.text]; ok {
			return fmt.Errorf("UNCHANGED names a bound value, not a declared variable")
		}
	}
	for _, a := range n.args {
		if err := unshadowed(a, bound); err != nil {
			return err
		}
	}
	return nil
}

func (m *module) variableList(n *node, stack []string) ([]string, error) {
	if n.op == "group" {
		return m.variableList(n.args[0], stack)
	}
	if n.op == "name" {
		if slices.Contains(m.variables, n.text) {
			return []string{n.text}, nil
		}
		d, ok := m.defs[n.text]
		if !ok || len(d.params) != 0 || slices.Contains(stack, n.text) || len(stack) >= 128 {
			return nil, fmt.Errorf("expected a variable or tuple of variables: %s", n.text)
		}
		return m.variableList(d.body, append(slices.Clone(stack), n.text))
	}
	if n.op != "tuple" {
		return nil, fmt.Errorf("expected a variable or tuple of variables")
	}
	var names []string
	for _, a := range n.args {
		part, err := m.variableList(a, stack)
		if err != nil {
			return nil, err
		}
		for _, name := range part {
			if slices.Contains(names, name) {
				return nil, fmt.Errorf("repeated variable %s", name)
			}
			names = append(names, name)
		}
	}
	return names, nil
}

func (m *module) fullSubscript(name string) error {
	names, err := m.variableList(&node{op: "name", text: name}, nil)
	if err != nil {
		return err
	}
	if len(names) != len(m.variables) {
		return fmt.Errorf("subscript %s must contain every declared variable exactly once", name)
	}
	return nil
}

func (m *module) expressionLevel(n *node, c context) (int, error) {
	bound := map[string]int{}
	for name := range c.locals {
		bound[name] = 0
	}
	return m.level(n, bound, c.stack)
}
