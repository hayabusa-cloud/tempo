// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type token struct {
	text string
	line int
}

type node struct {
	op, text string
	args     []*node
	names    []string
	line     int
}

type definition struct {
	params []string
	body   *node
}

type module struct {
	name               string
	constants          []string
	variables          []string
	defs               map[string]definition
	assumes            []*node
	naturals, integers bool
	levels             map[string]int
}

type configuration struct {
	constants  map[string]*node
	spec       string
	invariants []string
	properties []string
}

type parser struct {
	tokens   []token
	pos      int
	depth    int
	oldDepth int
}

func lex(source string) ([]token, error) {
	// Canonicalize line endings before scanning comments and string literals.
	source = strings.ReplaceAll(source, "\r\n", "\n")
	source = strings.ReplaceAll(source, "\r", "\n")
	var out []token
	line := 1
	for i := 0; i < len(source); {
		c := source[i]
		if c == '\n' {
			line++
			i++
			continue
		}
		if c == ' ' || c == '\t' {
			i++
			continue
		}
		if strings.HasPrefix(source[i:], "\\*") {
			for i < len(source) && source[i] != '\n' {
				i++
			}
			continue
		}
		if strings.HasPrefix(source[i:], "(*") {
			depth := 1
			i += 2
			for i < len(source) && depth > 0 {
				switch {
				case strings.HasPrefix(source[i:], "(*"):
					depth++
					i += 2
				case strings.HasPrefix(source[i:], "*)"):
					depth--
					i += 2
				default:
					if source[i] == '\n' {
						line++
					}
					i++
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("line %d: unclosed comment", line)
			}
			continue
		}
		start := i
		if strings.HasPrefix(source[i:], "----") || strings.HasPrefix(source[i:], "====") {
			for i < len(source) && source[i] == c {
				i++
			}
			out = append(out, token{strings.Repeat(string(c), 4), line})
			continue
		}
		if c == '"' {
			i++
			closed := false
			for i < len(source) {
				if source[i] == '\n' {
					return nil, fmt.Errorf("line %d: newline in string", line)
				}
				if source[i] == '\\' {
					i++
					if i == len(source) || !strings.ContainsRune("\\\"tnfr", rune(source[i])) {
						return nil, fmt.Errorf("line %d: unsupported string escape", line)
					}
					i++
					continue
				}
				if source[i] == '"' {
					i++
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, fmt.Errorf("line %d: unclosed string", line)
			}
		} else if identStart(c) {
			i++
			for i < len(source) && (identStart(source[i]) || source[i] >= '0' && source[i] <= '9') {
				i++
			}
		} else if c >= '0' && c <= '9' {
			i++
			for i < len(source) && source[i] >= '0' && source[i] <= '9' {
				i++
			}
			if i < len(source) && identStart(source[i]) {
				return nil, fmt.Errorf("line %d: digit-leading identifiers are unsupported", line)
			}
		} else {
			// These longer TLA+ lexemes must not split into supported tokens.
			for _, op := range []string{"\\intersect", "\\AA", "\\EE", "--"} {
				if strings.HasPrefix(source[i:], op) {
					return nil, fmt.Errorf("line %d: unsupported token %s", line, op)
				}
			}
			matched := false
			for _, op := range []string{"<=>", "|->", "----", "====", "\\in", "\\notin", "\\E", "\\A", "/\\", "\\/", "==", "=>", "->", "[]", "<>", "<<", ">>", "<=", ">=", "/=", ".."} {
				if strings.HasPrefix(source[i:], op) {
					i += len(op)
					matched = true
					break
				}
			}
			if !matched {
				if !strings.ContainsRune("(){}[],:.!'@=#+-*~<>", rune(c)) {
					return nil, fmt.Errorf("line %d: unsupported character %q", line, c)
				}
				i++
			}
		}
		out = append(out, token{source[start:i], line})
	}
	return append(out, token{"<end>", line}), nil
}

func identStart(c byte) bool   { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
func (p *parser) peek() string { return p.tokens[p.pos].text }
func (p *parser) take() token {
	t := p.tokens[p.pos]
	if t.text != "<end>" {
		p.pos++
	}
	return t
}
func (p *parser) accept(s string) bool {
	if p.peek() == s {
		p.take()
		return true
	}
	return false
}
func (p *parser) expect(s string) error {
	if p.accept(s) {
		return nil
	}
	return p.errorf("expected %q, found %q", s, p.peek())
}
func (p *parser) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s", p.tokens[p.pos].line, fmt.Sprintf(format, args...))
}
func (p *parser) name() (string, error) {
	t := p.take()
	if t.text == "<end>" || !identStart(t.text[0]) || !strings.ContainsAny(t.text, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return "", fmt.Errorf("line %d: expected name, found %q", t.line, t.text)
	}
	if reserved(t.text) {
		return "", fmt.Errorf("line %d: reserved name %s", t.line, t.text)
	}
	return t.text, nil
}

func reserved(s string) bool {
	switch s {
	case "TRUE", "FALSE", "BOOLEAN", "String", "MODULE", "EXTENDS", "CONSTANT", "CONSTANTS", "VARIABLE", "VARIABLES", "ASSUME", "ASSUMPTION", "AXIOM", "THEOREM", "INSTANCE", "LOCAL", "RECURSIVE", "WITH", "IF", "THEN", "ELSE", "CASE", "OTHER", "LET", "IN", "EXCEPT", "UNCHANGED", "ENABLED", "CHOOSE", "DOMAIN", "SUBSET", "UNION":
		return true
	case "ACTION", "LAMBDA", "STATE", "TEMPORAL", "NEW", "PROPOSITION", "LEMMA", "COROLLARY", "PROOF", "PROVE", "BY", "ONLY", "DEFINE", "DEF", "DEFS", "USE", "HIDE", "HAVE", "OBVIOUS", "OMITTED", "TAKE", "QED", "PICK", "WITNESS", "SUFFICES":
		return true
	}
	return strings.HasPrefix(s, "WF_") || strings.HasPrefix(s, "SF_")
}
func (p *parser) names() ([]string, error) {
	var names []string
	for {
		n, e := p.name()
		if e != nil {
			return nil, e
		}
		names = append(names, n)
		if !p.accept(",") {
			return names, nil
		}
	}
}

func precedence(s string) (low, high int) {
	switch s {
	case "=>":
		return 1, 1
	case "<=>":
		return 2, 2
	case "/\\", "\\/":
		return 3, 3
	case "~":
		return 4, 4
	case "[]", "<>", "UNCHANGED":
		return 4, 15
	case "=", "#", "/=", "<", ">", "<=", ">=", "\\in", "\\notin":
		return 5, 5
	case "..":
		return 9, 9
	case "+":
		return 10, 10
	case "-":
		return 11, 11
	case "unary -":
		return 12, 12
	case "*":
		return 13, 13
	case "'":
		return 15, 15
	case ".":
		return 17, 17
	}
	return 0, 0
}

func leftAssociative(s string) bool {
	switch s {
	case "/\\", "\\/", "+", "-", "*":
		return true
	}
	return false
}

func (p *parser) expr(parent string) (*node, error) {
	p.depth++
	defer func() { p.depth-- }()
	if p.depth > 256 {
		return nil, p.errorf("expression nesting limit exceeded")
	}
	parentLow, parentHigh := precedence(parent)
	t := p.take()
	n := &node{op: "name", text: t.text, line: t.line}
	var err error
	switch t.text {
	case "<end>":
		return nil, p.errorf("expected expression")
	case "(":
		var inner *node
		inner, err = p.expr("")
		if err == nil {
			err = p.expect(")")
		}
		n = &node{op: "group", args: []*node{inner}, line: t.line}
	case "~", "-", "[]", "<>", "UNCHANGED":
		var inner *node
		prefix := t.text
		if t.text == "-" {
			prefix = "unary -"
			low, _ := precedence(prefix)
			if parentLow > low {
				return nil, p.errorf("precedence conflict between %s and %s; use parentheses", parent, prefix)
			}
		}
		inner, err = p.expr(prefix)
		n = &node{op: t.text, args: []*node{inner}, line: t.line}
	case "/\\", "\\/":
		return nil, p.errorf("Boolean lists depend on indentation; use infix operators and parentheses")
	case "{", "<<":
		n.op = "set"
		close := "}"
		if t.text == "<<" {
			n.op = "tuple"
			close = ">>"
		}
		if !p.accept(close) {
			for {
				var a *node
				a, err = p.expr("")
				if err != nil {
					break
				}
				n.args = append(n.args, a)
				if !p.accept(",") {
					err = p.expect(close)
					break
				}
			}
		}
	case "[":
		n, err = p.bracket(t.line)
	case "IF":
		n.op = "if"
		for _, sep := range []string{"THEN", "ELSE", ""} {
			var a *node
			a, err = p.expr("")
			if err != nil {
				break
			}
			n.args = append(n.args, a)
			if sep != "" {
				err = p.expect(sep)
				if err != nil {
					break
				}
			}
		}
	case "CASE":
		n.op = "case"
		for {
			var guard, body *node
			if p.accept("OTHER") {
				if len(n.args) == 0 {
					return nil, p.errorf("CASE requires a guarded arm before OTHER")
				}
				guard = &node{op: "other", line: t.line}
			} else {
				guard, err = p.expr("")
			}
			if err != nil {
				break
			}
			if err = p.expect("->"); err != nil {
				break
			}
			body, err = p.expr("")
			if err != nil {
				break
			}
			n.args = append(n.args, guard, body)
			if !p.accept("[]") {
				break
			}
			if guard.op == "other" {
				return nil, p.errorf("OTHER must be the final CASE arm")
			}
		}
	case "LET":
		n.op = "let"
		for {
			var name string
			name, err = p.name()
			if err != nil {
				break
			}
			if err = p.expect("=="); err != nil {
				break
			}
			var a *node
			a, err = p.expr("")
			if err != nil {
				break
			}
			n.names = append(n.names, name)
			n.args = append(n.args, a)
			if p.accept("IN") {
				a, err = p.expr("")
				n.args = append(n.args, a)
				break
			}
		}
	case "\\E", "\\A":
		n.op = t.text
		var name string
		name, err = p.name()
		n.text = name
		if err == nil {
			err = p.expect("\\in")
		}
		if err == nil {
			var domain *node
			domain, err = p.expr("")
			n.args = append(n.args, domain)
		}
		if err == nil {
			err = p.expect(":")
		}
		if err == nil {
			var body *node
			body, err = p.expr("")
			n.args = append(n.args, body)
		}
	case "@":
		if p.oldDepth == 0 {
			return nil, p.errorf("@ is only defined inside an EXCEPT replacement")
		}
		n.op = "old"
	default:
		if t.text[0] == '"' {
			n.op = "string"
			n.text, err = strconv.Unquote(t.text)
		} else if unicode.IsDigit(rune(t.text[0])) {
			n.op = "integer"
		} else if !identStart(t.text[0]) {
			return nil, fmt.Errorf("line %d: unsupported expression %q", t.line, t.text)
		}
	}
	if err != nil {
		return nil, err
	}
	for {
		op := p.peek()
		switch op {
		case "~", "[]", "<>", "UNCHANGED":
			// Prefix operators cannot continue an expression; [] also ends a CASE arm.
			return n, nil
		}
		low, high := precedence(op)
		if low != 0 {
			if parentLow > high || parent == op && leftAssociative(op) {
				break
			}
			if parentHigh >= low {
				return nil, p.errorf("precedence conflict between %s and %s; use parentheses", parent, op)
			}
		}
		switch op {
		case ".":
			p.take()
			name, e := p.name()
			if e != nil {
				return nil, e
			}
			n = &node{op: "field", text: name, args: []*node{n}, line: t.line}
			continue
		case "'":
			p.take()
			if n.op != "name" {
				return nil, p.errorf("only declared variables may be primed")
			}
			n = &node{op: "prime", text: n.text, line: t.line}
			continue
		case "(":
			if n.op != "name" {
				return nil, p.errorf("only named operators may be called")
			}
			p.take()
			n.op = "call"
			if p.peek() == ")" {
				return nil, p.errorf("omit parentheses for a zero-argument operator")
			}
			for {
				a, e := p.expr("")
				if e != nil {
					return nil, e
				}
				n.args = append(n.args, a)
				if !p.accept(",") {
					if e = p.expect(")"); e != nil {
						return nil, e
					}
					break
				}
			}
			continue
		}
		if low == 0 {
			break
		}
		p.take()
		right, e := p.expr(op)
		if e != nil {
			return nil, e
		}
		n = &node{op: "binary", text: op, args: []*node{n, right}, line: t.line}
	}
	return n, nil
}

func (p *parser) bracket(line int) (*node, error) {
	if p.peek() != "<end>" && identStart(p.peek()[0]) && p.pos+1 < len(p.tokens) && (p.tokens[p.pos+1].text == "|->" || p.tokens[p.pos+1].text == ":") {
		n := &node{op: "record", line: line}
		sep := p.tokens[p.pos+1].text
		if sep == ":" {
			n.op = "recordset"
		}
		for {
			name, e := p.name()
			if e != nil {
				return nil, e
			}
			if e = p.expect(sep); e != nil {
				return nil, e
			}
			a, e := p.expr("")
			if e != nil {
				return nil, e
			}
			n.names = append(n.names, name)
			n.args = append(n.args, a)
			if !p.accept(",") {
				if e = p.expect("]"); e != nil {
					return nil, e
				}
				return n, nil
			}
		}
	}
	base, e := p.expr("")
	if e != nil {
		return nil, e
	}
	if p.accept("EXCEPT") {
		n := &node{op: "except", args: []*node{base}, line: line}
		for {
			if e = p.expect("!"); e != nil {
				return nil, e
			}
			if e = p.expect("."); e != nil {
				return nil, e
			}
			name, e := p.name()
			if e != nil {
				return nil, e
			}
			for p.accept(".") {
				part, err := p.name()
				if err != nil {
					return nil, err
				}
				name += "." + part
			}
			if e = p.expect("="); e != nil {
				return nil, e
			}
			p.oldDepth++
			a, e := p.expr("")
			p.oldDepth--
			if e != nil {
				return nil, e
			}
			n.names = append(n.names, name)
			n.args = append(n.args, a)
			if !p.accept(",") {
				if e = p.expect("]"); e != nil {
					return nil, e
				}
				return n, nil
			}
		}
	}
	if e = p.expect("]"); e != nil {
		return nil, e
	}
	sub, e := p.name()
	if e != nil {
		return nil, e
	}
	if !strings.HasPrefix(sub, "_") || len(sub) == 1 {
		return nil, p.errorf("expected full-variable action subscript")
	}
	return &node{op: "actionbox", text: sub[1:], args: []*node{base}, line: line}, nil
}

func parseModule(source string) (*module, error) {
	tokens, e := lex(source)
	if e != nil {
		return nil, e
	}
	p := parser{tokens: tokens}
	if e = p.expect("----"); e != nil {
		return nil, e
	}
	if e = p.expect("MODULE"); e != nil {
		return nil, e
	}
	header, e := p.name()
	if e != nil {
		return nil, e
	}
	if e = p.expect("----"); e != nil {
		return nil, e
	}
	m := &module{name: header, defs: map[string]definition{}}
	for p.peek() != "====" && p.peek() != "<end>" {
		switch p.peek() {
		case "EXTENDS":
			if m.naturals || len(m.defs)+len(m.constants)+len(m.variables)+len(m.assumes) > 0 {
				return nil, p.errorf("EXTENDS must occur once before declarations")
			}
			p.take()
			names, err := p.names()
			if err != nil {
				return nil, err
			}
			for _, name := range names {
				if name != "Naturals" && name != "Integers" {
					return nil, p.errorf("unsupported module %s", name)
				}
				m.naturals = true
				if name == "Integers" {
					m.integers = true
				}
			}
		case "CONSTANT", "CONSTANTS":
			p.take()
			names, err := p.names()
			if err != nil {
				return nil, err
			}
			m.constants = append(m.constants, names...)
		case "VARIABLE", "VARIABLES":
			p.take()
			names, err := p.names()
			if err != nil {
				return nil, err
			}
			m.variables = append(m.variables, names...)
		case "ASSUME":
			p.take()
			a, err := p.expr("")
			if err != nil {
				return nil, err
			}
			if err := syntaxBound(a); err != nil {
				return nil, err
			}
			if err := m.bindings(a, map[string]bool{}); err != nil {
				return nil, err
			}
			l, err := m.level(a, nil, nil)
			if err != nil {
				return nil, err
			}
			if l != 0 {
				return nil, p.errorf("ASSUME must be a constant expression")
			}
			m.assumes = append(m.assumes, a)
		default:
			name, err := p.name()
			if err != nil {
				return nil, err
			}
			d := definition{}
			if p.accept("(") {
				d.params, err = p.names()
				if err != nil {
					return nil, err
				}
				if err = p.expect(")"); err != nil {
					return nil, err
				}
			}
			if err = p.expect("=="); err != nil {
				return nil, err
			}
			params := map[string]bool{}
			for _, name := range d.params {
				if err := m.bindName(name, params); err != nil {
					return nil, err
				}
			}
			d.body, err = p.expr("")
			if err != nil {
				return nil, err
			}
			if _, ok := m.defs[name]; ok {
				return nil, p.errorf("duplicate operator %s", name)
			}
			if err := syntaxBound(d.body); err != nil {
				return nil, err
			}
			if err := m.bindings(d.body, params); err != nil {
				return nil, err
			}
			m.defs[name] = d
			bound := map[string]int{}
			for _, param := range d.params {
				bound[param] = 1
			}
			if _, err := m.level(d.body, bound, []string{name}); err != nil {
				return nil, err
			}
		}
	}
	if e = p.expect("===="); e != nil {
		return nil, e
	}
	if e = p.expect("<end>"); e != nil {
		return nil, e
	}
	seen := map[string]bool{}
	for _, name := range append(append([]string{}, m.constants...), m.variables...) {
		if name == "Nat" || name == "Int" {
			return nil, fmt.Errorf("reserved standard name %s", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("duplicate declaration %s", name)
		}
		seen[name] = true
		if _, ok := m.defs[name]; ok {
			return nil, fmt.Errorf("operator and variable/constant share name %s", name)
		}
	}
	for name := range m.defs {
		if name == "Nat" || name == "Int" {
			return nil, fmt.Errorf("reserved standard name %s", name)
		}
	}
	if len(m.variables) == 0 {
		return nil, fmt.Errorf("at least one state variable is required")
	}
	for _, d := range m.defs {
		if err := syntaxBound(d.body); err != nil {
			return nil, err
		}
	}
	for _, a := range m.assumes {
		if err := syntaxBound(a); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func syntaxBound(n *node) error {
	type item struct {
		node  *node
		depth int
	}
	stack := []item{{n, 0}}
	for len(stack) > 0 {
		i := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if i.depth > 256 {
			return fmt.Errorf("syntax nesting bound 256 exceeded")
		}
		if i.node.op == "record" || i.node.op == "recordset" {
			seen := map[string]bool{}
			for _, name := range i.node.names {
				if seen[name] {
					return fmt.Errorf("duplicate record field %s", name)
				}
				seen[name] = true
			}
		}
		for _, a := range i.node.args {
			stack = append(stack, item{a, i.depth + 1})
		}
	}
	return nil
}

func configKeyword(s string) bool {
	switch s {
	case "CONSTANT", "CONSTANTS", "SPECIFICATION", "INVARIANT", "INVARIANTS", "PROPERTY", "PROPERTIES":
		return true
	}
	return false
}

func parseConfig(source string) (*configuration, error) {
	tokens, e := lex(source)
	if e != nil {
		return nil, e
	}
	p := parser{tokens: tokens}
	c := &configuration{constants: map[string]*node{}}
	for p.peek() != "<end>" {
		kind := p.take().text
		switch kind {
		case "SPECIFICATION":
			if c.spec != "" {
				return nil, p.errorf("duplicate specification")
			}
			c.spec, e = p.name()
			if e != nil {
				return nil, e
			}
		case "CONSTANT", "CONSTANTS":
			start := p.pos
			for p.peek() != "<end>" && !configKeyword(p.peek()) {
				name, err := p.name()
				if err != nil {
					return nil, err
				}
				if err = p.expect("="); err != nil {
					return nil, err
				}
				a, err := p.expr("")
				if err != nil {
					return nil, err
				}
				if _, ok := c.constants[name]; ok {
					return nil, p.errorf("duplicate constant %s", name)
				}
				c.constants[name] = a
			}
			if p.pos == start {
				return nil, p.errorf("empty %s list", kind)
			}
		case "INVARIANT", "INVARIANTS", "PROPERTY", "PROPERTIES":
			start := p.pos
			for p.peek() != "<end>" && !configKeyword(p.peek()) {
				name, err := p.name()
				if err != nil {
					return nil, err
				}
				if strings.HasPrefix(kind, "INVARIANT") {
					c.invariants = append(c.invariants, name)
				} else {
					c.properties = append(c.properties, name)
				}
			}
			if p.pos == start {
				return nil, p.errorf("empty %s list", kind)
			}
		default:
			return nil, p.errorf("unsupported configuration directive %s", kind)
		}
	}
	if c.spec == "" || len(c.invariants)+len(c.properties) == 0 {
		return nil, fmt.Errorf("a specification and at least one requested check are required")
	}
	for _, a := range c.constants {
		if err := syntaxBound(a); err != nil {
			return nil, err
		}
	}
	return c, nil
}
