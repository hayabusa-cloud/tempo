// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResponseAndRecurrence(t *testing.T) {
	const prefix = "Init == x = 0\nNext == (x = 0 /\\ x' = 1) \\/ (x = 1 /\\ x' = 2)\nSpec == Init /\\ [][Next]_vars /\\ WF_vars(Next)\n"
	for _, tc := range []struct{ name, property, status string }{
		{"past response", "[](x = 2 => <>(x = 1))", "violation"},
		{"response now", "[](x = 1 => <>(x = 1))", "ok"},
		{"later response", "[](x = 1 => <>(x = 2))", "ok"},
		{"unreachable trigger", "[](x = 3 => <>(x = 4))", "ok"},
		{"recurrence after one response", "[]<>(x = 1)", "violation"},
		{"persistent goal", "[]<>(x = 2)", "ok"},
		{"always goal", "[]<>TRUE", "ok"},
		{"never goal", "[]<>FALSE", "violation"},
		{"invalid trigger", "[](x + 1 => <>TRUE)", "not_checked"},
		{"invalid goal", "[](TRUE => <>(x + 1))", "not_checked"},
		{"temporal trigger", "[]((<>TRUE) => <>TRUE)", "not_checked"},
		{"nested goal", "[](TRUE => <>[]TRUE)", "not_checked"},
		{"eventually always", "<>[]TRUE", "not_checked"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := model(t, scalar(prefix+"P == "+tc.property), "SPECIFICATION Spec\nPROPERTY P")
			if r.Status != tc.status {
				t.Fatalf("got %+v, want %s", r, tc.status)
			}
			if tc.name == "past response" || tc.name == "recurrence after one response" {
				c := r.Counterexample
				if c == nil || c.Trigger == nil || *c.Trigger != 2 || len(c.Stem) != 3 {
					t.Fatalf("missing the unrestricted prefix before the trigger: %+v", r)
				}
				for i, state := range c.Stem {
					x, err := state["x"].(json.Number).Int64()
					if err != nil || x != int64(i) {
						t.Fatalf("wrong prefix: %v", c.Stem)
					}
				}
				for _, state := range c.Cycle {
					if state["x"] != json.Number("2") {
						t.Fatalf("goal appeared after the trigger: %v", c.Cycle)
					}
				}
			}
		})
	}
	aliases := prefix + "Q == x = 2\nEventuallyQ == <>Q\nImplication == x = 1 => EventuallyQ\nP == []Implication\nR == []EventuallyQ"
	if r := model(t, scalar(aliases), "SPECIFICATION Spec\nPROPERTIES P R"); r.Status != "ok" || len(r.Checked) != 2 {
		t.Fatalf("property aliases failed: %+v", r)
	}
	multiple := strings.Replace(prefix, "Init == x = 0", "Init == x \\in {0,3}", 1)
	if r := model(t, scalar(multiple+"P == [](x = 3 => <>(x = 2))"), "SPECIFICATION Spec\nPROPERTY P"); r.Status != "violation" {
		t.Fatalf("missed another initial state's request: %+v", r)
	}
}

func TestService(t *testing.T) {
	for _, tc := range []struct{ name, status, property string }{
		{"Service", "ok", ""}, {"Owner", "violation", "Response"}, {"Starved", "violation", "Recurring"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := invoke(t, "-config", "../../spec/"+tc.name+".cfg", "../../spec/Service.tla")
			if r.Status != tc.status || r.States != 6 || r.Transitions != 14 || !r.Complete {
				t.Fatalf("unexpected service result: %+v", r)
			}
			if tc.property != "" && (r.Counterexample == nil || r.Counterexample.Property != tc.property) {
				t.Fatalf("missing expected witness: %+v", r)
			}
		})
	}
}

func TestProofOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Service.lean")
	r := invoke(t, "-proof", path, "-config", "../../spec/Service.cfg", "../../spec/Service.tla")
	if r.Status != "ok" || r.Proof != path {
		t.Fatalf("proof output failed: %+v", r)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"import Tempo", "Module SHA-256:", "Configuration SHA-256:", "certificate_1.Valid model", "Tempo.Response", "Tempo.InfinitelyOften", "collectAxioms"} {
		if !strings.Contains(string(data), required) {
			t.Fatalf("missing proof content %q", required)
		}
	}
	for _, source := range []struct{ name, label string }{{"Service.tla", "Module"}, {"Service.cfg", "Configuration"}} {
		input, err := os.ReadFile(filepath.Join("../../spec", source.name))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), fmt.Sprintf("%s SHA-256: %x", source.label, sha256.Sum256(input))) {
			t.Fatalf("incorrect source identity for %s", source.name)
		}
	}
	if r := invoke(t, "-proof", path, "-config", "../../spec/Service.cfg", "../../spec/Service.tla"); r.Status != "not_checked" || !strings.Contains(r.Error, "file exists") {
		t.Fatalf("existing proof was overwritten: %+v", r)
	}
	still, err := os.ReadFile(path)
	if err != nil || string(still) != string(data) {
		t.Fatal("existing proof content changed")
	}
	for _, cfg := range []string{"Owner", "Starved"} {
		absent := filepath.Join(t.TempDir(), "Absent.lean")
		if r := invoke(t, "-proof", absent, "-config", "../../spec/"+cfg+".cfg", "../../spec/Service.tla"); r.Status != "violation" || r.Proof != "" {
			t.Fatalf("violation produced positive proof: %+v", r)
		}
		if _, err := os.Stat(absent); !os.IsNotExist(err) {
			t.Fatal("violation wrote proof output")
		}
	}
	if r := invoke(t, "-proof", filepath.Join(t.TempDir(), "missing", "Proof.lean"), "-config", "../../spec/Service.cfg", "../../spec/Service.tla"); r.Status != "not_checked" {
		t.Fatalf("unwritable proof reported success: %+v", r)
	}
	for _, flags := range [][]string{nil, {"-max-states", "1"}} {
		absent := filepath.Join(t.TempDir(), "Incomplete.lean")
		r := model(t, scalar("Init == x = 0\nNext == x' = 1\nSpec == Init /\\ [][Next]_vars"),
			"SPECIFICATION Spec", append([]string{"-proof", absent}, flags...)...)
		if r.Status != "not_checked" || r.Proof != "" {
			t.Fatalf("empty or incomplete check produced evidence: %+v", r)
		}
		if _, err := os.Stat(absent); !os.IsNotExist(err) {
			t.Fatal("empty or incomplete check wrote a proof")
		}
	}
}

// The full harness opts into kernel checks after building the local proof library.
func TestKernelCertificates(t *testing.T) {
	if os.Getenv("TEMPO_CHECK_LEAN") != "1" {
		t.Skip("run ./verify.sh certificates for kernel validation")
	}
	proofRoot, err := filepath.Abs("../../proofs")
	if err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, path string, valid bool) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, filepath.Join(proofRoot, "run.sh"), "lake", "env", "lean", path)
		cmd.Dir = proofRoot
		cmd.WaitDelay = time.Second
		out, err := cmd.CombinedOutput()
		if ctx.Err() != nil || (err == nil) != valid {
			t.Fatalf("kernel result: %v\n%s", err, out)
		}
		if !valid && !strings.Contains(string(out), "error:") {
			t.Fatalf("expected proof rejection, not a process failure: %s", out)
		}
	}
	for _, tc := range []struct{ config, module string }{{"Progress", "Lifetime"}, {"Service", "Service"}} {
		t.Run(tc.config, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), tc.config+".lean")
			if r := invoke(t, "-proof", path, "-config", "../../spec/"+tc.config+".cfg", "../../spec/"+tc.module+".tla"); r.Status != "ok" {
				t.Fatal(r)
			}
			check(t, path, true)
		})
	}
	path := filepath.Join(t.TempDir(), "Control.lean")
	const body = "Init == x = 0\nNext == x < 2 /\\ x' = x + 1\nSpec == Init /\\ [][Next]_vars /\\ WF_vars(Next)\nP == <>(x = 2)"
	if r := model(t, scalar(body), "SPECIFICATION Spec\nPROPERTY P", "-proof", path); r.Status != "ok" {
		t.Fatal(r)
	}
	check(t, path, true)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct{ name, old, replacement string }{
		{"missing region", "region := fun i => (if i.val < 1 then true else (if i.val < 2 then true else false))", "region := fun _ => false"},
		{"increasing rank", "rank := fun i => (if i.val < 1 then 1 else 0)", "rank := fun i => (if i.val < 1 then 0 else 1)"},
		{"disabled blocker", "blocker := fun _ => 0", "blocker := fun _ => 7"},
		{"assumed validity", "theorem valid_0 : certificate_0.Valid model := by\n  apply Tempo.of_checkFinite\n  decide +kernel", "axiom valid_0 : certificate_0.Valid model"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			bad := strings.Replace(string(data), mutation.old, mutation.replacement, 1)
			if bad == string(data) {
				t.Fatalf("control did not alter the artifact: %s", mutation.old)
			}
			file := filepath.Join(t.TempDir(), "Invalid.lean")
			if err := os.WriteFile(file, []byte(bad), 0600); err != nil {
				t.Fatal(err)
			}
			check(t, file, false)
		})
	}
}
