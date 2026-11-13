// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import "fmt"

type counterexample struct {
	Property string `json:"property"`
	Kind     string `json:"kind"`
	Stem     []any  `json:"stem"`
	Cycle    []any  `json:"cycle,omitempty"`
	Trigger  *int   `json:"trigger,omitempty"`
}

type result struct {
	Status         string          `json:"status"`
	Specification  string          `json:"specification,omitempty"`
	States         int             `json:"states"`
	Transitions    int             `json:"transitions"`
	Complete       bool            `json:"complete"`
	Checked        []string        `json:"checked,omitempty"`
	Counterexample *counterexample `json:"counterexample,omitempty"`
	Error          string          `json:"error,omitempty"`
	Proof          string          `json:"proof,omitempty"`
	proof          *proofData
}

func predicate(e *evaluator, n *node, s state) (bool, error) {
	v, err := e.eval(n, context{before: s})
	if err != nil {
		return false, err
	}
	return v.truth()
}

func containsState(states []state, w state) bool {
	key := w.key()
	for _, v := range states {
		if v.key() == key {
			return true
		}
	}
	return false
}

// Replay interprets the source relations again. It does not trust graph edges,
// fairness labels, SCC membership, or the search's predicate cache.
func replay(e *evaluator, s *specification, p property, stem, cycle []state, trigger int) error {
	if len(stem) == 0 {
		return fmt.Errorf("counterexample has no initial state")
	}
	initial, err := e.successors(s.initial, nil, true)
	if err != nil {
		return err
	}
	if !containsState(initial, stem[0]) {
		return fmt.Errorf("counterexample does not start in Init")
	}
	for _, path := range [][]state{stem, cycle} {
		for i := 0; i+1 < len(path); i++ {
			if path[i].key() == path[i+1].key() {
				continue
			}
			next, err := e.successors(s.next, path[i], false)
			if err != nil {
				return err
			}
			if !containsState(next, path[i+1]) {
				return fmt.Errorf("counterexample contains an invalid edge")
			}
		}
	}
	if p.kind == invariantProperty {
		yes, err := predicate(e, p.predicate, stem[len(stem)-1])
		if err != nil {
			return err
		}
		if yes {
			return fmt.Errorf("counterexample does not violate the invariant")
		}
		return nil
	}
	if len(cycle) < 2 || cycle[0].key() != cycle[len(cycle)-1].key() || stem[len(stem)-1].key() != cycle[0].key() {
		return fmt.Errorf("counterexample is not a closed lasso")
	}
	if trigger < 0 || trigger >= len(stem) || (p.kind == eventualProperty && trigger != 0) {
		return fmt.Errorf("counterexample has an invalid trigger position")
	}
	if p.kind == responseProperty {
		yes, err := predicate(e, p.trigger, stem[trigger])
		if err != nil {
			return err
		}
		if !yes {
			return fmt.Errorf("counterexample does not contain the response trigger")
		}
	}
	for _, path := range [][]state{stem[trigger:], cycle} {
		for _, v := range path {
			yes, err := predicate(e, p.predicate, v)
			if err != nil {
				return err
			}
			if yes {
				return fmt.Errorf("eventual predicate already holds in the counterexample")
			}
		}
	}
	for _, action := range s.fairness {
		witnessed := false
		for i := 0; i+1 < len(cycle); i++ {
			next, err := e.successors(action, cycle[i], false)
			if err != nil {
				return err
			}
			enabled := false
			occurs := false
			for _, v := range next {
				if v.key() != cycle[i].key() {
					enabled = true
					if v.key() == cycle[i+1].key() {
						occurs = true
					}
				}
			}
			if !enabled || occurs {
				witnessed = true
				break
			}
		}
		if !witnessed {
			return fmt.Errorf("counterexample violates weak fairness")
		}
	}
	return nil
}

func check(m *module, c *configuration, b bounds, proof bool) result {
	r := result{Status: "not_checked", Specification: c.spec}
	e, err := configure(m, c, b)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	s, err := m.specification(c)
	if err != nil {
		r.Error = err.Error()
		return r
	}
	g, err := explore(e, s, b)
	if g != nil {
		r.States = len(g.vertices)
		r.Transitions = g.transitions
	}
	if err != nil {
		r.Error = err.Error()
		return r
	}
	r.Complete = true
	if proof {
		r.proof = &proofData{graph: g}
	}
	parents := make([]int, len(g.vertices))
	for i, v := range g.vertices {
		parents[i] = v.parent
	}
	for _, p := range s.checks {
		holds := make([]bool, len(g.vertices))
		seeds := make([]bool, len(g.vertices))
		if p.kind == eventualProperty {
			for _, id := range g.initial {
				seeds[id] = true
			}
		}
		var w *witness
		var cert *certificate
		for i, v := range g.vertices {
			yes, err := predicate(e, p.predicate, v.state)
			if err != nil {
				r.Error = fmt.Sprintf("property %s at state %d: %s", p.name, i, err)
				return r
			}
			holds[i] = yes
			if p.kind == recurrenceProperty {
				seeds[i] = true
			}
			if p.kind == responseProperty {
				seed, err := predicate(e, p.trigger, v.state)
				if err != nil {
					r.Error = fmt.Sprintf("trigger %s at state %d: %s", p.name, i, err)
					return r
				}
				seeds[i] = seed
			}
			if p.kind == invariantProperty && !yes && w == nil {
				w = &witness{stem: prefix(parents, i)}
			}
		}
		if p.kind != invariantProperty {
			w, cert, err = g.liveness(holds, seeds)
			if err != nil {
				r.Error = err.Error()
				return r
			}
		}
		if w != nil {
			var stem, cycle []state
			trace := &counterexample{Property: p.name, Kind: "safety"}
			for _, id := range w.stem {
				v := g.vertices[id].state
				stem = append(stem, v)
				trace.Stem = append(trace.Stem, v.display())
			}
			for _, id := range w.cycle {
				v := g.vertices[id].state
				cycle = append(cycle, v)
				trace.Cycle = append(trace.Cycle, v.display())
			}
			if p.kind != invariantProperty {
				trace.Kind = "liveness"
				trace.Trigger = &w.trigger
			}
			if err := replay(e, s, p, stem, cycle, w.trigger); err != nil {
				r.Error = "counterexample replay: " + err.Error()
				return r
			}
			r.Status = "violation"
			r.Counterexample = trace
			return r
		}
		if proof {
			r.proof.checks = append(r.proof.checks, proofCheck{property: p, holds: holds, seeds: seeds, certificate: cert})
		}
		r.Checked = append(r.Checked, p.name)
	}
	r.Status = "ok"
	return r
}
