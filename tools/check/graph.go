// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"slices"
)

type edge struct {
	to   int
	fair []bool
}
type vertex struct {
	state   state
	edges   []edge
	enabled []bool
	parent  int
}
type graph struct {
	vertices    []vertex
	initial     []int
	transitions int
}
type bounds struct{ states, transitions, values, steps int }

func explore(e *evaluator, s *specification, b bounds) (*graph, error) {
	g := &graph{}
	ids := map[string]int{}
	sorts := map[string]string{}
	add := func(v state, parent int) (int, error) {
		key := v.key()
		if id, ok := ids[key]; ok {
			return id, nil
		}
		if err := admitSort(value{kind: "record", fields: v}, "state", sorts); err != nil {
			return 0, err
		}
		if len(g.vertices) >= b.states {
			return 0, fmt.Errorf("state bound %d exhausted", b.states)
		}
		id := len(g.vertices)
		ids[key] = id
		g.vertices = append(g.vertices, vertex{state: v, parent: parent})
		return id, nil
	}
	initial, err := e.successors(s.initial, nil, true)
	if err != nil {
		return g, fmt.Errorf("initial predicate: %w", err)
	}
	if len(initial) == 0 {
		return g, fmt.Errorf("initial predicate has no states")
	}
	for _, v := range initial {
		id, err := add(v, -1)
		if err != nil {
			return g, err
		}
		g.initial = append(g.initial, id)
	}
	for id := 0; id < len(g.vertices); id++ {
		v := g.vertices[id].state
		next, err := e.successors(s.next, v, false)
		if err != nil {
			return g, fmt.Errorf("next relation at state %d: %w", id, err)
		}
		allowed := map[string]bool{}
		for _, w := range next {
			allowed[w.key()] = true
		}
		fair := make([]map[string]bool, len(s.fairness))
		enabled := make([]bool, len(s.fairness))
		for i, action := range s.fairness {
			fair[i] = map[string]bool{}
			ws, err := e.successors(action, v, false)
			if err != nil {
				return g, fmt.Errorf("fairness action %d at state %d: %w", i, id, err)
			}
			for _, w := range ws {
				key := w.key()
				if key == v.key() {
					continue
				}
				if !allowed[key] {
					return g, fmt.Errorf("fairness action %d admits an edge outside Next at state %d", i, id)
				}
				enabled[i] = true
				fair[i][key] = true
			}
		}
		// [Next]_vars always permits stuttering, even at a terminal state.
		if !allowed[v.key()] {
			next = append(next, v)
		}
		var edges []edge
		for _, w := range next {
			if g.transitions >= b.transitions {
				return g, fmt.Errorf("transition bound %d exhausted", b.transitions)
			}
			to, err := add(w, id)
			if err != nil {
				return g, err
			}
			labels := make([]bool, len(fair))
			for i := range fair {
				labels[i] = fair[i][w.key()]
			}
			edges = append(edges, edge{to: to, fair: labels})
			g.transitions++
		}
		g.vertices[id].edges = edges
		g.vertices[id].enabled = enabled
	}
	return g, nil
}

func prefix(parent []int, to int) []int {
	var out []int
	for to >= 0 {
		out = append(out, to)
		to = parent[to]
	}
	slices.Reverse(out)
	return out
}

// components uses an explicit DFS stack: graph size does not become Go call
// depth. Its input is a selected reachable subgraph.
func (g *graph) components(allowed []bool) [][]int {
	visited := make([]bool, len(g.vertices))
	var order []int
	type frame struct{ id, next int }
	for root := range g.vertices {
		if !allowed[root] || visited[root] {
			continue
		}
		visited[root] = true
		stack := []frame{{id: root}}
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.next == len(g.vertices[top.id].edges) {
				order = append(order, top.id)
				stack = stack[:len(stack)-1]
				continue
			}
			to := g.vertices[top.id].edges[top.next].to
			top.next++
			if allowed[to] && !visited[to] {
				visited[to] = true
				stack = append(stack, frame{id: to})
			}
		}
	}
	reverse := make([][]int, len(g.vertices))
	for from, v := range g.vertices {
		if allowed[from] {
			for _, ed := range v.edges {
				if allowed[ed.to] {
					reverse[ed.to] = append(reverse[ed.to], from)
				}
			}
		}
	}
	clear(visited)
	var out [][]int
	for _, root := range slices.Backward(order) {
		if visited[root] {
			continue
		}
		visited[root] = true
		component := []int{}
		stack := []int{root}
		for len(stack) > 0 {
			id := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			component = append(component, id)
			for _, to := range reverse[id] {
				if !visited[to] {
					visited[to] = true
					stack = append(stack, to)
				}
			}
		}
		slices.Sort(component)
		out = append(out, component)
	}
	return out
}

// path returns a shortest path within a component, including both endpoints.
func (g *graph) path(from, to int, inside []bool) ([]int, error) {
	if from == to {
		return []int{from}, nil
	}
	parents := make([]int, len(g.vertices))
	for i := range parents {
		parents[i] = -2
	}
	parents[from] = -1
	queue := []int{from}
	for head := 0; head < len(queue); head++ {
		id := queue[head]
		if id == to {
			return prefix(parents, to), nil
		}
		for _, ed := range g.vertices[id].edges {
			if inside[ed.to] && parents[ed.to] == -2 {
				parents[ed.to] = id
				queue = append(queue, ed.to)
			}
		}
	}
	return nil, fmt.Errorf("internal error: component path does not exist")
}

type witness struct {
	stem, cycle []int
	trigger     int
}

func (g *graph) liveness(holds, seeds []bool) (*witness, *certificate, error) {
	allowed := make([]bool, len(g.vertices))
	parents := make([]int, len(g.vertices))
	for i := range parents {
		parents[i] = -2
	}
	queue := []int{}
	for id, seed := range seeds {
		if seed && !holds[id] {
			allowed[id] = true
			parents[id] = -1
			queue = append(queue, id)
		}
	}
	for head := 0; head < len(queue); head++ {
		id := queue[head]
		for _, ed := range g.vertices[id].edges {
			if !holds[ed.to] && !allowed[ed.to] {
				allowed[ed.to] = true
				parents[ed.to] = id
				queue = append(queue, ed.to)
			}
		}
	}
	components := g.components(allowed)
	cert := &certificate{region: allowed, rank: make([]int, len(g.vertices)), blocker: make([]int, len(components))}
	inside := make([]bool, len(g.vertices))
	for i, component := range components {
		rank := len(components) - 1 - i
		for _, id := range component {
			inside[id] = true
			cert.rank[id] = rank
		}
		// A witness is a disabled vertex or an internal nonstuttering edge.
		type requirement struct{ from, to int }
		var required []requirement
		fair := true
		for action := range g.vertices[component[0]].enabled {
			found := false
			for _, id := range component {
				if !g.vertices[id].enabled[action] {
					required = append(required, requirement{id, id})
					found = true
					break
				}
			}
			if !found {
				for _, id := range component {
					for _, ed := range g.vertices[id].edges {
						if inside[ed.to] && ed.fair[action] {
							required = append(required, requirement{id, ed.to})
							found = true
							break
						}
					}
					if found {
						break
					}
				}
			}
			if !found {
				fair = false
				cert.blocker[rank] = action
				break
			}
		}
		if !fair {
			for _, id := range component {
				inside[id] = false
			}
			continue
		}
		entry := component[0]
		cycle := []int{entry}
		for _, r := range required {
			p, err := g.path(cycle[len(cycle)-1], r.from, inside)
			if err != nil {
				return nil, nil, err
			}
			cycle = append(cycle, p[1:]...)
			if r.to != r.from {
				cycle = append(cycle, r.to)
			}
		}
		p, err := g.path(cycle[len(cycle)-1], entry, inside)
		if err != nil {
			return nil, nil, err
		}
		cycle = append(cycle, p[1:]...)
		if len(cycle) == 1 {
			cycle = append(cycle, entry)
		}
		suffix := prefix(parents, entry)
		initialParents := make([]int, len(g.vertices))
		for id, v := range g.vertices {
			initialParents[id] = v.parent
		}
		stem := prefix(initialParents, suffix[0])
		trigger := len(stem) - 1
		stem = append(stem, suffix[1:]...)
		return &witness{stem: stem, cycle: cycle, trigger: trigger}, nil, nil
	}
	if err := cert.verify(g, holds, seeds); err != nil {
		return nil, nil, err
	}
	return nil, cert, nil
}
