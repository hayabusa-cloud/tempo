// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var executable string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "tempo-check-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	executable = filepath.Join(dir, "tempo-check")
	args := []string{"build", "-buildvcs=false", "-race", "-o", executable}
	if os.Getenv("TEMPO_CHECK_COVERDIR") != "" {
		args = append(args, "-cover", "-covermode=atomic")
	}
	args = append(args, ".")
	cmd := exec.Command("go", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type report struct {
	Status              string `json:"status"`
	States, Transitions int
	Complete            bool
	Checked             []string
	Error               string
	Proof               string
	Counterexample      *struct {
		Property, Kind string
		Stem, Cycle    []map[string]any
		Trigger        *int
	}
}

func invoke(t testing.TB, args ...string) report {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0 halt_on_error=1")
	if dir := os.Getenv("TEMPO_CHECK_COVERDIR"); dir != "" {
		cmd.Env = append(cmd.Env, "GOCOVERDIR="+dir)
	}
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			t.Fatal(err)
		}
	}
	var r report
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber()
	if err := dec.Decode(&r); err != nil {
		t.Fatalf("invalid report: %v\n%s", err, out)
	}
	want := map[string]int{"ok": 0, "violation": 1, "not_checked": 2}
	expected, ok := want[r.Status]
	if !ok || code != expected {
		t.Fatalf("exit %d and status %s disagree: %s", code, r.Status, out)
	}
	return r
}

func model(t testing.TB, source, config string, flags ...string) report {
	t.Helper()
	dir := t.TempDir()
	tla := filepath.Join(dir, "Test.tla")
	cfg := filepath.Join(dir, "Test.cfg")
	if err := os.WriteFile(tla, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	return invoke(t, append(append([]string{"-config", cfg}, flags...), tla)...)
}

func scalar(body string) string {
	return "---- MODULE Test ----\nEXTENDS Integers\nVARIABLE x\nvars == <<x>>\n" + body + "\n====\n"
}

func TestLifetime(t *testing.T) {
	for _, tc := range []struct {
		name, status, property string
		states                 int
	}{
		{"Safety", "ok", "", 324}, {"Progress", "ok", "", 324}, {"Unsafe", "violation", "ReturnedAfterSettlement", 432}, {"Unfair", "violation", "EventuallyReturned", 324},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := invoke(t, "-config", "../../spec/"+tc.name+".cfg", "../../spec/Lifetime.tla")
			if r.Status != tc.status || r.States != tc.states || !r.Complete {
				t.Fatalf("unexpected report: %+v", r)
			}
			if tc.property != "" && (r.Counterexample == nil || r.Counterexample.Property != tc.property) {
				t.Fatalf("missing expected counterexample: %+v", r)
			}
			if tc.name == "Progress" && len(r.Checked) != 6 {
				t.Fatalf("not all checks ran: %+v", r)
			}
		})
	}
}

func TestSemanticBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, body, check, status string }{
		{"exact integers", `Init == x = 9223372036854775808
Next == x' = x
Spec == Init /\ [][Next]_vars
P == x + 1 = 9223372036854775809`, "INVARIANT P", "ok"},
		{"grouped temporal comparison", `Init == x = 1
Next == x' = 0
Spec == Init /\ [][Next]_vars
P == <> (x = 1)`, "PROPERTY P", "ok"},
		{"negation membership precedence", `Init == x = TRUE
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == ~ x \in BOOLEAN`, "INVARIANT P", "violation"},
		{"explicit quantifier scope", `Init == x = 0
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == (\A item \in {} : TRUE) /\ FALSE`, "INVARIANT P", "violation"},
		{"adjacent membership name", `Numbers == {0}
Init == x = 0
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == x \inNumbers`, "INVARIANT P", "ok"},
		{"keyword prefixes remain names", `ACTIONSReady == TRUE
TEMPORALSReady == TRUE
Init == x = 0
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == ACTIONSReady /\ TEMPORALSReady`, "INVARIANT P", "ok"},
		{"separated minus tokens", `Init == x = 0
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == 1-(* separate tokens *)-2 = 3`, "INVARIANT P", "ok"},
		{"goal before bad cycle", `Init == x = 0
Next == (x = 0 /\ x' = 1) \/ (x = 1 /\ x' = 2)
Spec == Init /\ [][Next]_vars /\ WF_vars(Next)
P == <> (x = 1)`, "PROPERTY P", "ok"},
		{"multiple initial states", `Init == x \in {0,1}
Next == x' = 1
Spec == Init /\ [][Next]_vars /\ WF_vars(Next)
P == <> (x = 1)`, "PROPERTY P", "ok"},
		{"stuttering is not a fairness occurrence", `Init == x = 0
Next == x' = 1
Idle == UNCHANGED x
Spec == Init /\ [][Next]_vars /\ WF_vars(Idle)
P == <> (x = 1)`, "PROPERTY P", "violation"},
		{"guards precede assignments", `Init == x = [n |-> 0]
Next == (x' = x.missing /\ FALSE) \/ (UNCHANGED x)
Spec == Init /\ [][Next]_vars
P == x.n = 0`, "INVARIANT P", "ok"},
		{"simultaneous record updates", `Init == x = [a |-> 0, b |-> 1]
Next == x' = [x EXCEPT !.a = x.b, !.b = x.a]
Spec == Init /\ [][Next]_vars
P == x.a + x.b = 1`, "INVARIANT P", "ok"},
		{"old nested field", `Init == x = [a |-> [n |-> 0]]
Next == x' = [x EXCEPT !.a.n = IF @ = 0 THEN 1 ELSE 0]
Spec == Init /\ [][Next]_vars
P == x.a.n \in {0,1}`, "INVARIANT P", "ok"},
		{"finite record enumeration", `Init == x \in [n : 0..1, b : BOOLEAN]
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == x.n \in Nat`, "INVARIANT P", "ok"},
		{"quantified predicate and LET", `Init == x = {1,2}
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == LET a == 1 b == a + 1 IN \A n \in x : n >= a /\ n <= b`, "INVARIANT P", "ok"},
		{"lexical operator parameters", `G(a) == a + 1
F(a) == G(a) * 2
a == 7
Init == x = F(2)
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == x = 6`, "INVARIANT P", "ok"},
		{"later declaration preserves local binding", `F == LET a == 1 IN a
a == 7
Init == x = F
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == x = 1`, "INVARIANT P", "ok"},
		{"record field postcondition", `Init == x = [n |-> 0]
Next == x' = [n |-> 1] /\ x'.n = 1
Spec == Init /\ [][Next]_vars
P == x.n \in {0,1}`, "INVARIANT P", "ok"},
		{"deadlock still stutters", `Init == x = 0
Next == FALSE
Spec == Init /\ [][Next]_vars
P == <> (x = 1)`, "PROPERTY P", "violation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := model(t, scalar(tc.body), "SPECIFICATION Spec\n"+tc.check)
			if r.Status != tc.status {
				t.Fatalf("got %+v, want %s", r, tc.status)
			}
			if r.Status == "violation" && (r.Counterexample == nil || r.Counterexample.Property != "P") {
				t.Fatalf("missing counterexample: %+v", r)
			}
		})
	}
}

func TestOperatorPrecedence(t *testing.T) {
	for _, tc := range []struct{ name, expression, directive, status string }{
		{"eventual comparison", `<> (x = 0)`, "PROPERTY", "ok"},
		{"always comparison", `[] (x = 0)`, "PROPERTY", "ok"},
		{"recurring comparison", `[]<> (x = 0)`, "PROPERTY", "ok"},
		{"temporal negation chain", `<>~ x = 1`, "PROPERTY", "ok"},
		{"temporal field selection", `<>[ok |-> TRUE].ok`, "PROPERTY", "ok"},
		{"temporal conditional", `<> IF TRUE THEN x = 0 ELSE FALSE`, "PROPERTY", "ok"},
		{"CASE arm separator", `CASE x = 0 -> TRUE [] OTHER -> FALSE`, "INVARIANT", "ok"},
		{"equivalence after implication", `FALSE => TRUE <=> FALSE`, "INVARIANT", "ok"},
		{"equivalence before implication", `FALSE <=> FALSE => TRUE`, "INVARIANT", "ok"},
		{"left associative subtraction", `10 - 3 - 2 = 5`, "INVARIANT", "ok"},
		{"grouped subtraction", `10 - (3 - 2) = 9`, "INVARIANT", "ok"},
		{"grouped negative factor", `1 * (-2) = -2`, "INVARIANT", "ok"},
		{"negated product", `-1 * 2 = -2`, "INVARIANT", "ok"},
		{"eventual comparison conflict", `<> x = 0`, "PROPERTY", "not_checked"},
		{"always comparison conflict", `[] x = 0`, "PROPERTY", "not_checked"},
		{"recurring comparison conflict", `[]<> x = 0`, "PROPERTY", "not_checked"},
		{"temporal prime conflict", `<> x'`, "PROPERTY", "not_checked"},
		{"unchanged comparison conflict", `UNCHANGED x = x`, "INVARIANT", "not_checked"},
		{"ungrouped negative factor", `1 * -2 = -2`, "INVARIANT", "not_checked"},
		{"mixed conjunction", `TRUE /\ FALSE \/ TRUE`, "INVARIANT", "not_checked"},
		{"mixed disjunction", `TRUE \/ FALSE /\ TRUE`, "INVARIANT", "not_checked"},
		{"chained implication", `TRUE => FALSE => TRUE`, "INVARIANT", "not_checked"},
		{"chained equivalence", `TRUE <=> FALSE <=> TRUE`, "INVARIANT", "not_checked"},
		{"chained comparison", `0 = 0 = TRUE`, "INVARIANT", "not_checked"},
		{"chained range", `0..1..2 = {0,1,2}`, "INVARIANT", "not_checked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := scalar("Init == x = 0\nNext == UNCHANGED x\nSpec == Init /\\ [][Next]_vars\nP == " + tc.expression)
			r := model(t, source, "SPECIFICATION Spec\n"+tc.directive+" P")
			if r.Status != tc.status {
				t.Fatalf("unexpected precedence result: %+v", r)
			}
			if tc.status == "not_checked" && (r.Complete || !strings.Contains(r.Error, "precedence")) {
				t.Fatalf("expected a precedence conflict before exploration: %+v", r)
			}
		})
	}
}

func TestRejectedSpecifications(t *testing.T) {
	base := `Init == x = 0
Next == x' = 1
Spec == Init /\ [][Next]_vars
P == TRUE`
	for _, tc := range []struct{ name, old, next string }{
		{"unconstrained state", "Next == x' = 1", "Next == TRUE"},
		{"primed dependency", "Next == x' = 1", "Next == x' = x' + 1"},
		{"initial dependency", "Init == x = 0", "Init == x = x + 1"},
		{"infinite domain", "Init == x = 0", `Init == x \in Nat`},
		{"duplicate assignment", "Next == x' = 1", `Next == x' = 1 /\ x' = 1`},
		{"recursion", "Next == x' = 1", "Next == Next"},
		{"ambiguous CASE", "Next == x' = 1", "Next == x' = CASE TRUE -> 0 [] TRUE -> 1"},
		{"CASE without a branch", "Next == x' = 1", "Next == x' = CASE FALSE -> 0"},
		{"CASE only OTHER", "P == TRUE", "P == CASE OTHER -> TRUE"},
		{"unused misplaced OTHER", "P == TRUE", "Unused == CASE FALSE -> 0 [] OTHER -> 1 [] TRUE -> 2\nP == TRUE"},
		{"overlapping EXCEPT", "Next == x' = 1", "Next == x' = [[n |-> 0] EXCEPT !.n = 1, !.n = 2]"},
		{"cross-sort record equality", "P == TRUE", "P == [n |-> 0] = [n |-> FALSE]"},
		{"cross-sort set", "Init == x = 0", "Init == x = {0, FALSE}"},
		{"false assumption", "Init == x = 0", "ASSUME FALSE\nInit == x = 0"},
		{"unknown operator", "Next == x' = 1", "Next == Missing"},
		{"fairness outside Next", `[][Next]_vars`, `[][Next]_vars /\ WF_vars(x' = 2)`},
		{"strong fairness", `[][Next]_vars`, `[][Next]_vars /\ SF_vars(Next)`},
		{"mixed Boolean syntax", "P == TRUE", `P == TRUE /\ FALSE \/ TRUE`},
		{"Boolean list scope", "P == TRUE", "P == /\\ \\A item \\in {} : TRUE\n     /\\ FALSE"},
		{"Boolean list", "P == TRUE", `P == /\ TRUE`},
		{"disjunction list", "P == TRUE", `P == \/ TRUE`},
		{"intersection token", "P == TRUE", "tersect == {0}\nP == x \\intersect"},
		{"temporal universal token", "P == TRUE", `P == \AA \in {} : FALSE`},
		{"temporal existential token", "P == TRUE", `P == \EE \in {0} : E = 0`},
		{"double minus token", "P == TRUE", "P == 1--2 = 3"},
		{"numeric identifier suffix", "Init == x = 0", "Init == x = 0Tail == TRUE"},
		{"numeric underscore suffix", "Init == x = 0", "Init == x = 0_Tail == TRUE"},
		{"unknown standard module", "EXTENDS Integers", "EXTENDS FiniteSets"},
		{"missing standard import", "EXTENDS Integers", ""},
		{"chained implication", "P == TRUE", "P == TRUE => FALSE => TRUE"},
		{"duplicate parameter", "P == TRUE", "F(a,a) == a\nP == TRUE"},
		{"parameter shadows state", "P == TRUE", "F(x) == x\nP == TRUE"},
		{"parameter shadows operator", "P == TRUE", "F(Init) == Init\nP == TRUE"},
		{"LET shadows operator", "P == TRUE", "P == LET Init == TRUE IN Init"},
		{"LET shadows outer local", "P == TRUE", "P == LET a == TRUE IN LET a == FALSE IN a"},
		{"quantifier shadows state", "P == TRUE", `P == \A x \in {0} : x = 0`},
		{"quantifier shadows parameter", "P == TRUE", "F(a) == \\A a \\in {0} : a = 0\nP == TRUE"},
		{"parameter shadows import", "P == TRUE", "F(Nat) == Nat\nP == TRUE"},
		{"value applied as operator", "P == TRUE", "F(a) == a(1)\nP == TRUE"},
		{"empty parameter list", "P == TRUE", "P() == TRUE"},
		{"empty argument list", "P == TRUE", "P == Init()"},
		{"reserved declaration", "P == TRUE", "AXIOM == TRUE\nP == TRUE"},
		{"reserved proof word", "P == TRUE", "PROOF == TRUE\nP == TRUE"},
		{"reserved action alias", "P == TRUE", "ACTIONS == TRUE\nP == TRUE"},
		{"reserved temporal alias", "P == TRUE", "TEMPORALS == TRUE\nP == TRUE"},
		{"identifier without a letter", "P == TRUE", "___ == TRUE\nP == TRUE"},
		{"predefined String", "P == TRUE", "String == TRUE\nP == TRUE"},
		{"unused unknown operator", "P == TRUE", "Unused == Missing\nP == TRUE"},
		{"unused temporal action", "P == TRUE", "Unused == <>(x' = 0)\nP == TRUE"},
		{"forward operator reference", "P == TRUE", "A == B\nB == TRUE\nP == TRUE"},
		{"unused duplicate field", "P == TRUE", "Unused == [a |-> 1, a |-> 2]\nP == TRUE"},
		{"state sort change", "Next == x' = 1", "Next == x' = TRUE"},
		{"old value outside update", "P == TRUE", "P == @ = 0"},
		{"shadowed variable tuple", "Next == x' = 1", "A(vars) == UNCHANGED vars\nNext == A(0)"},
		{"incomplete variable subscript", "vars == <<x>>", "vars == <<>>"},
		{"repeated variable subscript", "vars == <<x>>", "vars == <<x,x>>"},
		{"nested temporal property", "P == TRUE", "P == <>[] (x = 1)"},
		{"trailer is not ignored", "====", "====\nBogus == TRUE"},
		{"duplicate module", "====", "---- MODULE Second ----\n===="},
		{"empty initial set", "Init == x = 0", `Init == x \in {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := scalar(base)
			if tc.name == "missing standard import" {
				source = strings.ReplaceAll(source, "Next == x' = 1", "Next == x' = x + 1")
			}
			source = strings.Replace(source, tc.old, tc.next, 1)
			r := model(t, source, "SPECIFICATION Spec\nINVARIANT P")
			if r.Status != "not_checked" || r.Error == "" {
				t.Fatalf("invalid specification was accepted: %+v", r)
			}
		})
	}
	for _, cfg := range []string{"SPECIFICATION Spec", "SPECIFICATION Spec\nCONSTRAINT P\nINVARIANT P", "SPECIFICATION Spec\nPROPERTY P"} {
		if r := model(t, scalar(base), cfg); r.Status != "not_checked" {
			t.Fatalf("unsupported config accepted: %+v", r)
		}
	}
	for _, extra := range [][]string{{"-max-states", "1"}, {"-max-transitions", "1"}, {"-max-values", "0"}, {"-max-steps", "1"}, {"-unknown"}} {
		if r := model(t, scalar(base), "SPECIFICATION Spec\nINVARIANT P", extra...); r.Status != "not_checked" {
			t.Fatalf("limit or option accepted: %+v", r)
		}
	}
	for _, source := range []string{"", "(* ---- MODULE Fake ---- *)\n====", "---- MODULE Bad ----\n(* unclosed", scalar("Init == x = (\n")} {
		if r := model(t, source, "SPECIFICATION Spec\nINVARIANT P"); r.Status != "not_checked" {
			t.Fatalf("malformed syntax accepted: %+v", r)
		}
	}
}

func TestFiniteLimitsAreNotConstraints(t *testing.T) {
	for _, value := range []string{
		`[a : 0..3, b : 0..3]`,
		`<<[a : 0..3, b : 0..3]>>`,
		`[nested |-> [a : 0..3, b : 0..3]]`,
	} {
		source := scalar("Init == x = " + value + "\nNext == FALSE\nSpec == Init /\\ [][Next]_vars\nP == TRUE")
		if r := model(t, source, "SPECIFICATION Spec\nINVARIANT P", "-max-values", "16"); r.Status != "not_checked" || r.Complete {
			t.Fatalf("materialized value exceeded its bound: %+v", r)
		}
	}
	exact := scalar("Init == x = [a : 0..3, b : 0..3]\nNext == FALSE\nSpec == Init /\\ [][Next]_vars\nP == TRUE")
	for _, tc := range []struct {
		limit, status string
	}{{"48", "not_checked"}, {"49", "ok"}} {
		if r := model(t, exact, "SPECIFICATION Spec\nINVARIANT P", "-max-values", tc.limit); r.Status != tc.status {
			t.Fatalf("materialized size boundary %s: %+v", tc.limit, r)
		}
	}
	changing := scalar(`Init == x = {}
Next == (x' = {0}) \/ (x' = {TRUE})
Spec == Init /\ [][Next]_vars
P == TRUE`)
	if r := model(t, changing, "SPECIFICATION Spec\nINVARIANT P"); r.Status != "not_checked" {
		t.Fatalf("nested sort change was accepted: %+v", r)
	}
	source := scalar(`Init == x = 0
Next == x' = x + 1
Spec == Init /\ [][Next]_vars
P == x < 2`)
	r := model(t, source, "SPECIFICATION Spec\nINVARIANT P", "-max-states", "10")
	if r.Status != "not_checked" || r.Complete || r.States != 10 {
		t.Fatalf("invariant or bound pruned exploration: %+v", r)
	}
	source = scalar(`Init == x \in 0..10
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == TRUE`)
	if r := model(t, source, "SPECIFICATION Spec\nINVARIANT P", "-max-values", "3"); r.Status != "not_checked" {
		t.Fatalf("enumeration was truncated: %+v", r)
	}
	source = scalar("Init == x = 0\nNext == UNCHANGED x\nSpec == Init /\\ [][Next]_vars\nP == " + strings.Repeat("TRUE /\\ ", 300) + "TRUE")
	if r := model(t, source, "SPECIFICATION Spec\nINVARIANT P"); r.Status != "not_checked" {
		t.Fatalf("syntax depth bound missing: %+v", r)
	}
}

func TestCommentsAndModuleDelimiters(t *testing.T) {
	source := scalar(`(*
====
(* ---- MODULE Fake ---- *)
*)
Init == x = "a\"b"
Next == UNCHANGED x
Spec == Init /\ [][Next]_vars
P == x = "a\"b"`)
	if r := model(t, source, "SPECIFICATION Spec\nINVARIANT P"); r.Status != "ok" {
		t.Fatalf("comments changed syntax: %+v", r)
	}
}

func TestProgressAcrossManyComponents(t *testing.T) {
	const steps = 8000
	source := scalar(fmt.Sprintf(`Init == x = 0
Next == x < %d /\ x' = x + 1
Spec == Init /\ [][Next]_vars /\ WF_vars(Next)
P == <> (x = %d)`, steps, steps))
	r := model(t, source, "SPECIFICATION Spec\nPROPERTY P")
	if r.Status != "ok" || !r.Complete || r.States != steps+1 || r.Transitions != 2*steps+1 {
		t.Fatalf("finite chain did not close: %+v", r)
	}
}

// This oracle enumerates short closed walks directly. It has no SCC algorithm
// and independently evaluates fairness from the original integer edge masks.
func badFairWalk(edges [3][3]bool, labels [2][3][3]bool) bool {
	reachable := [2]bool{true, edges[0][1]}
	for start := range 2 {
		if !reachable[start] {
			continue
		}
		var visit func([]int) bool
		visit = func(path []int) bool {
			last := path[len(path)-1]
			if len(path) > 1 && last == start {
				fair := true
				for action := range 2 {
					witnessed := false
					for i := 0; i+1 < len(path); i++ {
						from, to := path[i], path[i+1]
						enabled := false
						for dst := range 3 {
							enabled = enabled || (dst != from && labels[action][from][dst])
						}
						witnessed = witnessed || !enabled || (from != to && labels[action][from][to])
					}
					fair = fair && witnessed
				}
				if fair {
					return true
				}
			}
			if len(path) == 7 {
				return false
			}
			for to := range 2 {
				if to == last || edges[last][to] {
					if visit(append(path, to)) {
						return true
					}
				}
			}
			return false
		}
		if visit([]int{start}) {
			return true
		}
	}
	return false
}

func TestWeakFairnessAgainstWalkOracle(t *testing.T) {
	pairs := [][2]int{{0, 1}, {1, 0}, {0, 2}, {1, 2}}
	for code := range 625 {
		remaining := code
		var edges [3][3]bool
		var labels [2][3][3]bool
		clauses := make([][]string, 3)
		for _, pair := range pairs {
			digit := remaining % 5
			remaining /= 5
			if digit == 0 {
				continue
			}
			from, to := pair[0], pair[1]
			edges[from][to] = true
			term := fmt.Sprintf("(x = %d /\\ x' = %d)", from, to)
			clauses[0] = append(clauses[0], term)
			for action := range 2 {
				if digit == 4 || digit == action+2 {
					labels[action][from][to] = true
					clauses[action+1] = append(clauses[action+1], term)
				}
			}
		}
		definitions := make([]string, 3)
		for i, c := range clauses {
			definitions[i] = "FALSE"
			if len(c) > 0 {
				definitions[i] = strings.Join(c, " \\/ ")
			}
		}
		body := fmt.Sprintf("Init == x = 0\nNext == %s\nA == %s\nB == %s\nSpec == Init /\\ [][Next]_vars /\\ WF_vars(A) /\\ WF_vars(B)\nP == <> (x = 2)", definitions[0], definitions[1], definitions[2])
		r := model(t, scalar(body), "SPECIFICATION Spec\nPROPERTY P")
		want := "ok"
		if badFairWalk(edges, labels) {
			want = "violation"
		}
		if r.Status != want {
			t.Fatalf("graph %d: got %+v, want %s\n%s", code, r, want, body)
		}
		if r.Counterexample != nil {
			verifyWalk(t, r, edges, labels)
		}
	}
}

func verifyWalk(t *testing.T, r report, edges [3][3]bool, labels [2][3][3]bool) {
	t.Helper()
	decode := func(path []map[string]any) []int {
		var out []int
		for _, s := range path {
			v, ok := s["x"].(json.Number)
			if !ok {
				t.Fatal("non-numeric trace state")
			}
			n, err := v.Int64()
			if err != nil || n < 0 || n > 1 {
				t.Fatalf("goal or invalid state in lasso: %v", v)
			}
			out = append(out, int(n))
		}
		return out
	}
	stem, cycle := decode(r.Counterexample.Stem), decode(r.Counterexample.Cycle)
	if len(stem) == 0 || stem[0] != 0 || len(cycle) < 2 || stem[len(stem)-1] != cycle[0] || cycle[0] != cycle[len(cycle)-1] {
		t.Fatal("invalid lasso endpoints")
	}
	for _, path := range [][]int{stem, cycle} {
		for i := 0; i+1 < len(path); i++ {
			if path[i] != path[i+1] && !edges[path[i]][path[i+1]] {
				t.Fatal("invalid lasso edge")
			}
		}
	}
	for action := range 2 {
		fair := false
		for i := 0; i+1 < len(cycle); i++ {
			from, to := cycle[i], cycle[i+1]
			enabled := false
			for dst := range 3 {
				enabled = enabled || (dst != from && labels[action][from][dst])
			}
			fair = fair || !enabled || (from != to && labels[action][from][to])
		}
		if !fair {
			t.Fatal("lasso is not weakly fair")
		}
	}
}

func BenchmarkLifetime(b *testing.B) {
	for b.Loop() {
		r := invoke(b, "-config", "../../spec/Progress.cfg", "../../spec/Lifetime.tla")
		if r.Status != "ok" {
			b.Fatal(r)
		}
	}
}
