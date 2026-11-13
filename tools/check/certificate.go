// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type certificate struct {
	region        []bool
	rank, blocker []int
}

// verify uses only graph edges and local certificate conditions, never SCCs.
// Enabledness is from the full graph, including edges that reach the goal.
func (c *certificate) verify(g *graph, holds, seeds []bool) error {
	if len(c.region) != len(g.vertices) || len(c.rank) != len(g.vertices) || len(holds) != len(g.vertices) || len(seeds) != len(g.vertices) {
		return fmt.Errorf("certificate dimensions do not match the graph")
	}
	for id, v := range g.vertices {
		if seeds[id] && !holds[id] && !c.region[id] {
			return fmt.Errorf("certificate omits seed %d", id)
		}
		if !c.region[id] {
			continue
		}
		rank := c.rank[id]
		if holds[id] || rank < 0 || rank >= len(c.blocker) {
			return fmt.Errorf("invalid certificate region or rank at state %d", id)
		}
		action := c.blocker[rank]
		if action < 0 || action >= len(v.enabled) || !v.enabled[action] {
			return fmt.Errorf("certificate task is not enabled at state %d", id)
		}
		for _, ed := range v.edges {
			if holds[ed.to] {
				continue
			}
			if !c.region[ed.to] || c.rank[ed.to] > rank {
				return fmt.Errorf("certificate region or rank is not preserved by %d -> %d", id, ed.to)
			}
			if c.rank[ed.to] == rank && ed.fair[action] {
				return fmt.Errorf("certificate task occurs without rank decrease on %d -> %d", id, ed.to)
			}
		}
	}
	return nil
}

type proofCheck struct {
	property
	holds, seeds []bool
	*certificate
}

type proofData struct {
	graph  *graph
	checks []proofCheck
}

func leanList[T any](out io.Writer, values []T, write func(io.Writer, T)) {
	fmt.Fprint(out, "[")
	for i, value := range values {
		if i > 0 {
			fmt.Fprint(out, ", ")
		}
		write(out, value)
	}
	fmt.Fprint(out, "]")
}

func leanValue[T any](out io.Writer, value T) { fmt.Fprint(out, value) }

// Balanced branches avoid linear table lookup during kernel reduction.
// Constant regions collapse without changing any encoded state value.
func constant[T comparable](values []T) bool {
	for _, value := range values[1:] {
		if value != values[0] {
			return false
		}
	}
	return true
}

func leanBranches[T comparable](out io.Writer, index string, offset int, values []T, write func(io.Writer, T)) {
	if constant(values) {
		write(out, values[0])
		return
	}
	mid := len(values) / 2
	fmt.Fprintf(out, "(if %s < %d then ", index, offset+mid)
	leanBranches(out, index, offset, values[:mid], write)
	fmt.Fprint(out, " else ")
	leanBranches(out, index, offset+mid, values[mid:], write)
	fmt.Fprint(out, ")")
}

func leanTable[T comparable](out io.Writer, values []T, write func(io.Writer, T)) {
	if constant(values) {
		fmt.Fprint(out, "fun _ => ")
		write(out, values[0])
		return
	}
	fmt.Fprint(out, "fun i => ")
	leanBranches(out, "i.val", 0, values, write)
}

func leanActions(out io.Writer, flags []bool) {
	fmt.Fprint(out, "[")
	separator := ""
	for j, present := range flags {
		if present {
			fmt.Fprintf(out, "%s%d", separator, j)
			separator = ", "
		}
	}
	fmt.Fprint(out, "]")
}

// write emits data and proofs for the exact explored graph. Kernel reduction
// checks each certificate; source interpretation is a separate trust boundary.
func (p *proofData) write(path string, moduleHash, configHash [32]byte) (err error) {
	if len(p.checks) == 0 {
		return fmt.Errorf("no properties to certify")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			os.Remove(path)
		}
	}()
	out := bufio.NewWriter(file)
	fmt.Fprintf(out, "import Tempo\nimport Lean\n\n-- Module SHA-256: %x\n-- Configuration SHA-256: %x\n", moduleHash, configHash)
	fmt.Fprint(out, "-- Claims concern this encoded graph, with fairness in specification order.\nset_option maxRecDepth 65536\nset_option maxHeartbeats 10000000\nnamespace TempoCheck\n\n")
	n := len(p.graph.vertices)
	for id, v := range p.graph.vertices {
		data, err := json.Marshal(v.state.display())
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "-- State %d: %s\n", id, data)
	}
	// Separate rows share edge data during kernel reduction.
	rows := make([]int, n)
	for id, v := range p.graph.vertices {
		rows[id] = id
		fmt.Fprintf(out, "def edges_%d : List (Fin %d × List Nat) := ", id, n)
		leanList(out, v.edges, func(out io.Writer, ed edge) {
			fmt.Fprintf(out, "(⟨%d, of_decide_eq_true rfl⟩, ", ed.to)
			leanActions(out, ed.fair)
			fmt.Fprint(out, ")")
		})
		fmt.Fprint(out, "\n")
		fmt.Fprintf(out, "def enabled_%d : List Nat := ", id)
		leanActions(out, v.enabled)
		fmt.Fprint(out, "\n")
	}
	fmt.Fprintf(out, "\ndef model : Tempo.Graph %d where\n  edges := ", n)
	leanTable(out, rows, func(out io.Writer, id int) { fmt.Fprintf(out, "edges_%d", id) })
	fmt.Fprint(out, "\n  enabled := ")
	leanTable(out, rows, func(out io.Writer, id int) { fmt.Fprintf(out, "enabled_%d", id) })
	fmt.Fprintf(out, "\n\ndef initial : List (Fin %d) := ", n)
	leanList(out, p.graph.initial, func(out io.Writer, id int) { fmt.Fprintf(out, "⟨%d, by decide⟩", id) })
	fmt.Fprint(out, "\n\n")
	for i, check := range p.checks {
		fmt.Fprintf(out, "-- Property %q\ndef goal_%d : Fin %d → Bool := ", check.name, i, n)
		leanTable(out, check.holds, leanValue[bool])
		fmt.Fprint(out, "\n")
		if check.kind == invariantProperty {
			fmt.Fprintf(out, "theorem check_%d : ∀ s, goal_%d s = true := by\n  apply Tempo.of_checkFinite\n  decide +kernel\n\n", i, i)
			continue
		}
		fmt.Fprintf(out, "def seed_%d : Fin %d → Bool := ", i, n)
		leanTable(out, check.seeds, leanValue[bool])
		fmt.Fprintf(out, "\ndef certificate_%d : Tempo.Certificate %d where\n  seed := seed_%d\n  goal := goal_%d\n  region := ", i, n, i, i)
		leanTable(out, check.region, leanValue[bool])
		fmt.Fprint(out, "\n  rank := ")
		leanTable(out, check.rank, leanValue[int])
		fmt.Fprint(out, "\n  blocker := ")
		if len(check.blocker) == 0 {
			fmt.Fprint(out, "fun _ => 0")
		} else if constant(check.blocker) {
			fmt.Fprintf(out, "fun _ => %d", check.blocker[0])
		} else {
			fmt.Fprint(out, "fun k => ")
			leanBranches(out, "k", 0, check.blocker, leanValue[int])
		}
		fmt.Fprintf(out, "\ntheorem valid_%d : certificate_%d.Valid model := by\n  apply Tempo.of_checkFinite\n  decide +kernel\n", i, i)
		fmt.Fprintf(out, "theorem check_%d (path : Nat → Fin %d) (steps : model.Path path) (fair : model.Fair path)\n", i, n)
		switch check.kind {
		case eventualProperty:
			fmt.Fprintf(out, "    (start : path 0 ∈ initial) : ∃ k, goal_%d (path k) = true := by\n", i)
			fmt.Fprintf(out, "  have covered : ∀ s ∈ initial, seed_%d s = true := by\n    letI := List.decidableBAll (fun s => seed_%d s = true) initial\n    decide\n", i, i)
			fmt.Fprintf(out, "  obtain ⟨k, _, goal⟩ := certificate_%d.sound model valid_%d path steps fair 0 (covered (path 0) start)\n  exact ⟨k, goal⟩\n\n", i, i)
		case responseProperty:
			fmt.Fprintf(out, "    : Tempo.Response (fun k => seed_%d (path k) = true) (fun k => goal_%d (path k) = true) :=\n  certificate_%d.sound model valid_%d path steps fair\n\n", i, i, i, i)
		case recurrenceProperty:
			fmt.Fprintf(out, "    : Tempo.InfinitelyOften (fun k => goal_%d (path k) = true) := by\n", i)
			fmt.Fprintf(out, "  have covered : ∀ s, seed_%d s = true := by\n    apply Tempo.of_checkFinite\n    decide +kernel\n  intro k\n  exact certificate_%d.sound model valid_%d path steps fair k (covered (path k))\n\n", i, i, i)
		}
	}
	fmt.Fprint(out, "end TempoCheck\n\nopen Lean Elab Command in\nrun_cmd do\n  let env ← getEnv\n  let required : Array Name := #[")
	for i := range p.checks {
		if i > 0 {
			fmt.Fprint(out, ", ")
		}
		fmt.Fprintf(out, "`TempoCheck.check_%d", i)
	}
	fmt.Fprint(out, "]\n  for name in required do\n    unless (env.find? name).any (fun info => info.isTheorem) do\n      throwError \"Missing certificate theorem: {name}\"\n    let axioms ← collectAxioms name\n    for axiomName in axioms do\n      unless #[`propext, `Classical.choice, `Quot.sound].contains axiomName do\n        throwError \"Unapproved axiom {axiomName} in {name}\"\n    logInfo m!\"{name}: {axioms}\"\n")
	return out.Flush()
}
