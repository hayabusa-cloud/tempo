// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

type value struct {
	kind   string
	b      bool
	n      *big.Int
	s      string
	list   []value
	fields map[string]value
}

func boolean(b bool) value  { return value{kind: "bool", b: b} }
func integer(n int64) value { return value{kind: "int", n: big.NewInt(n)} }

func fieldNames(fields map[string]value) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (v value) key() string {
	var b strings.Builder
	v.appendKey(&b)
	return b.String()
}

func (v value) appendKey(b *strings.Builder) {
	b.WriteString(v.kind)
	b.WriteByte(':')
	switch v.kind {
	case "bool":
		if v.b {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
	case "int":
		b.WriteString(v.n.String())
		b.WriteByte(';')
	case "string":
		b.WriteString(strconv.Quote(v.s))
	case "tuple", "set":
		b.WriteByte('[')
		for _, a := range v.list {
			a.appendKey(b)
		}
		b.WriteByte(']')
	case "record", "recordset":
		b.WriteByte('{')
		for _, name := range fieldNames(v.fields) {
			b.WriteString(strconv.Quote(name))
			v.fields[name].appendKey(b)
		}
		b.WriteByte('}')
	}
}

func (v value) display() any {
	switch v.kind {
	case "bool":
		return v.b
	case "int":
		return json.Number(v.n.String())
	case "string":
		return v.s
	case "record":
		r := map[string]any{}
		for name, a := range v.fields {
			r[name] = a.display()
		}
		return r
	case "tuple", "set":
		a := make([]any, len(v.list))
		for i, v := range v.list {
			a[i] = v.display()
		}
		if v.kind == "set" {
			return map[string]any{"set": a}
		}
		return a
	default:
		return v.kind
	}
}

func (v value) truth() (bool, error) {
	if v.kind != "bool" {
		return false, fmt.Errorf("expected Boolean, found %s", v.kind)
	}
	return v.b, nil
}

func getField(v value, name string) (value, error) {
	if v.kind != "record" {
		return value{}, fmt.Errorf("field %s selected from %s", name, v.kind)
	}
	a, ok := v.fields[name]
	if !ok {
		return value{}, fmt.Errorf("record has no field %s", name)
	}
	return a, nil
}

func getPath(v value, path []string) (value, error) {
	var err error
	for _, name := range path {
		v, err = getField(v, name)
		if err != nil {
			return value{}, err
		}
	}
	return v, nil
}

func replacePath(v value, path []string, replacement value) (value, error) {
	if len(path) == 0 {
		return replacement, nil
	}
	old, err := getField(v, path[0])
	if err != nil {
		return value{}, err
	}
	next, err := replacePath(old, path[1:], replacement)
	if err != nil {
		return value{}, err
	}
	fields := make(map[string]value, len(v.fields))
	for name, a := range v.fields {
		fields[name] = a
	}
	fields[path[0]] = next
	return value{kind: "record", fields: fields}, nil
}

type state map[string]value

func (s state) key() string  { return (value{kind: "record", fields: s}).key() }
func (s state) display() any { return (value{kind: "record", fields: s}).display() }

func admitSort(v value, path string, sorts map[string]string) error {
	if prior, ok := sorts[path]; ok && prior != v.kind {
		return fmt.Errorf("state value sort changes at %s: %s to %s", path, prior, v.kind)
	}
	sorts[path] = v.kind
	for _, name := range fieldNames(v.fields) {
		if err := admitSort(v.fields[name], path+"."+name, sorts); err != nil {
			return err
		}
	}
	for i, a := range v.list {
		part := "{}"
		if v.kind == "tuple" {
			part = fmt.Sprintf("[%d]", i)
		}
		if err := admitSort(a, path+part, sorts); err != nil {
			return err
		}
	}
	return nil
}

type context struct {
	before state
	after  state
	locals map[string]value
	old    *value
	stack  []string
}

func (c context) bind(name string, v value) context {
	locals := make(map[string]value, len(c.locals)+1)
	for k, a := range c.locals {
		locals[k] = a
	}
	locals[name] = v
	c.locals = locals
	return c
}

type evaluator struct {
	module          *module
	constants       map[string]value
	maxValues       int
	steps, maxSteps int
}

func (e *evaluator) spend() error {
	if e.steps >= e.maxSteps {
		return fmt.Errorf("evaluation step bound %d exhausted", e.maxSteps)
	}
	e.steps++
	return nil
}

func (e *evaluator) set(items []value) (value, error) {
	if len(items) > e.maxValues {
		return value{}, fmt.Errorf("finite-value bound exceeded")
	}
	unique := map[string]value{}
	for _, v := range items {
		a, err := e.canonical(v)
		if err != nil {
			return value{}, err
		}
		if len(unique) > 0 {
			for _, b := range unique {
				if err := comparableValues(a, b); err != nil {
					return value{}, err
				}
				if a.kind == "int" || a.kind == "bool" || a.kind == "string" {
					break
				}
			}
		}
		unique[a.key()] = a
	}
	keys := make([]string, 0, len(unique))
	for key := range unique {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	list := make([]value, 0, len(keys))
	for _, key := range keys {
		list = append(list, unique[key])
	}
	return value{kind: "set", list: list}, nil
}

func (e *evaluator) enumerate(v value) ([]value, error) {
	switch v.kind {
	case "set":
		return v.list, nil
	case "recordset":
		if _, _, err := e.enumerationSize(v, 0); err != nil {
			return nil, err
		}
		out := []value{{kind: "record", fields: map[string]value{}}}
		for _, name := range fieldNames(v.fields) {
			choices, err := e.enumerate(v.fields[name])
			if err != nil {
				return nil, err
			}
			if len(choices) > 0 && len(out) > e.maxValues/len(choices) {
				return nil, fmt.Errorf("record-domain enumeration bound exceeded")
			}
			var next []value
			for _, r := range out {
				for _, a := range choices {
					fields := make(map[string]value, len(r.fields)+1)
					for k, b := range r.fields {
						fields[k] = b
					}
					fields[name] = a
					next = append(next, value{kind: "record", fields: fields})
				}
			}
			out = next
		}
		return out, nil
	default:
		return nil, fmt.Errorf("cannot enumerate %s; a finite set is required", v.kind)
	}
}

func (e *evaluator) canonical(v value) (value, error) {
	if err := e.valueBound(v); err != nil {
		return value{}, err
	}
	return e.canonicalValue(v)
}

func (e *evaluator) valueBound(v value) error {
	_, err := e.valueSize(v, 0)
	return err
}

// Count the materialized tree, not the compact record-set descriptor. The
// arithmetic is bounded before multiplication, so expansion cannot overflow or
// allocate an oversized Cartesian product before discovering its limit.
func (e *evaluator) valueSize(v value, depth int) (int, error) {
	if depth > 256 {
		return 0, fmt.Errorf("value nesting bound exceeded")
	}
	if v.kind == "int" && v.n.BitLen() > 65536 {
		return 0, fmt.Errorf("integer bit bound 65536 exceeded")
	}
	if v.kind == "recordset" {
		_, nodes, err := e.enumerationSize(v, depth+1)
		if err != nil {
			return 0, err
		}
		if nodes >= e.maxValues {
			return 0, fmt.Errorf("materialized value size bound exceeded")
		}
		return 1 + nodes, nil
	}
	total := 1
	add := func(a value) error {
		n, err := e.valueSize(a, depth+1)
		if err != nil {
			return err
		}
		if n > e.maxValues-total {
			return fmt.Errorf("materialized value size bound exceeded")
		}
		total += n
		return nil
	}
	for _, a := range v.list {
		if err := add(a); err != nil {
			return 0, err
		}
	}
	for _, a := range v.fields {
		if err := add(a); err != nil {
			return 0, err
		}
	}
	return total, nil
}

// depth is the depth occupied by each enumerated element. nodes is the sum of
// their materialized sizes, excluding the surrounding set constructor.
func (e *evaluator) enumerationSize(v value, depth int) (count, nodes int, err error) {
	if depth > 256 {
		return 0, 0, fmt.Errorf("value nesting bound exceeded")
	}
	switch v.kind {
	case "set":
		for _, a := range v.list {
			n, err := e.valueSize(a, depth)
			if err != nil {
				return 0, 0, err
			}
			if n > e.maxValues-nodes {
				return 0, 0, fmt.Errorf("materialized enumeration bound exceeded")
			}
			nodes += n
		}
		return len(v.list), nodes, nil
	case "recordset":
		count, nodes = 1, 1 // One empty record, before adding fields.
		for _, name := range fieldNames(v.fields) {
			n, size, err := e.enumerationSize(v.fields[name], depth+1)
			if err != nil {
				return 0, 0, err
			}
			if n != 0 && nodes > e.maxValues/n {
				return 0, 0, fmt.Errorf("materialized enumeration bound exceeded")
			}
			base := nodes * n
			if count != 0 && size > (e.maxValues-base)/count {
				return 0, 0, fmt.Errorf("materialized enumeration bound exceeded")
			}
			nodes, count = base+count*size, count*n
		}
		return count, nodes, nil
	default:
		return 0, 0, fmt.Errorf("cannot enumerate %s; a finite set is required", v.kind)
	}
}

func (e *evaluator) canonicalValue(v value) (value, error) {
	switch v.kind {
	case "bool", "int", "string":
		return v, nil
	case "recordset":
		items, err := e.enumerate(v)
		if err != nil {
			return value{}, err
		}
		return e.set(items)
	case "set":
		return e.set(v.list)
	case "tuple":
		out := make([]value, len(v.list))
		for i, a := range v.list {
			b, err := e.canonicalValue(a)
			if err != nil {
				return value{}, err
			}
			out[i] = b
		}
		return value{kind: "tuple", list: out}, nil
	case "record":
		out := make(map[string]value, len(v.fields))
		for name, a := range v.fields {
			b, err := e.canonicalValue(a)
			if err != nil {
				return value{}, err
			}
			out[name] = b
		}
		return value{kind: "record", fields: out}, nil
	default:
		return value{}, fmt.Errorf("unsupported non-finite data value %s", v.kind)
	}
}

func (e *evaluator) equal(a, b value) (bool, error) {
	x, err := e.canonical(a)
	if err != nil {
		return false, err
	}
	y, err := e.canonical(b)
	if err != nil {
		return false, err
	}
	if err := comparableValues(x, y); err != nil {
		return false, err
	}
	return x.key() == y.key(), nil
}

// TLA+ does not prescribe equality between unrelated primitive sorts. Do not
// turn that unspecified comparison into false, including inside containers.
func comparableValues(a, b value) error {
	if a.kind != b.kind {
		return fmt.Errorf("cross-sort comparison is unsupported: %s and %s", a.kind, b.kind)
	}
	switch a.kind {
	case "record":
		for name, v := range a.fields {
			if w, ok := b.fields[name]; ok {
				if err := comparableValues(v, w); err != nil {
					return err
				}
			}
		}
	case "tuple":
		if len(a.list) == len(b.list) {
			for i, v := range a.list {
				if err := comparableValues(v, b.list[i]); err != nil {
					return err
				}
			}
		}
	case "set":
		for _, v := range a.list {
			for _, w := range b.list {
				if err := comparableValues(v, w); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (e *evaluator) member(a, domain value) (bool, error) {
	switch domain.kind {
	case "nat":
		if a.kind != "int" {
			return false, fmt.Errorf("Nat membership requires an integer")
		}
		return a.n.Sign() >= 0, nil
	case "integers":
		if a.kind != "int" {
			return false, fmt.Errorf("Int membership requires an integer")
		}
		return true, nil
	case "recordset":
		if a.kind != "record" {
			return false, fmt.Errorf("record-set membership requires a record")
		}
		if len(a.fields) != len(domain.fields) {
			return false, nil
		}
		for name, d := range domain.fields {
			v, ok := a.fields[name]
			if !ok {
				return false, nil
			}
			yes, err := e.member(v, d)
			if err != nil || !yes {
				return yes, err
			}
		}
		return true, nil
	case "set":
		for _, v := range domain.list {
			yes, err := e.equal(a, v)
			if err != nil {
				return false, err
			}
			if yes {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("membership requires a set, found %s", domain.kind)
	}
}
