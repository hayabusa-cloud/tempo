// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSourceLineEndings(t *testing.T) {
	const body = "Init == x = 0\nNext == UNCHANGED x\nSpec == Init /\\ [][Next]_vars\n"
	for _, tc := range []struct{ name, ending string }{
		{"LF", "\n"}, {"CRLF", "\r\n"}, {"CR", "\r"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("module comment", func(t *testing.T) {
				source := scalar(body + "P == TRUE \\* comment" + tc.ending + " /\\ FALSE")
				r := model(t, source, "SPECIFICATION Spec\nINVARIANT P\n")
				if r.Status != "violation" || r.Counterexample == nil || r.Counterexample.Property != "P" {
					t.Fatalf("comment hid a false conjunct: %+v", r)
				}
			})
			t.Run("configuration comment", func(t *testing.T) {
				config := "SPECIFICATION Spec\nINVARIANT Safe \\* comment" + tc.ending + "INVARIANT P\n"
				r := model(t, scalar(body+"Safe == TRUE\nP == FALSE"), config)
				if r.Status != "violation" || r.Counterexample == nil || r.Counterexample.Property != "P" || len(r.Checked) != 1 || r.Checked[0] != "Safe" {
					t.Fatalf("comment hid a requested check: %+v", r)
				}
			})
			t.Run("raw string newline", func(t *testing.T) {
				literal := "\"a" + tc.ending + "b\""
				r := model(t, scalar(body+"P == "+literal+" = "+literal), "SPECIFICATION Spec\nINVARIANT P\n")
				if r.Status != "not_checked" || r.Complete || !strings.Contains(r.Error, "newline in string") {
					t.Fatalf("raw string newline was not rejected: %+v", r)
				}
			})
			t.Run("diagnostic line", func(t *testing.T) {
				source := scalar(body + "(* outer\n(* nested *)\n*)\n\\* line comment\nP == Missing")
				line := strings.Count(source[:strings.Index(source, "Missing")], "\n") + 1
				source = strings.ReplaceAll(source, "\n", tc.ending)
				r := model(t, source, "SPECIFICATION Spec\nINVARIANT P\n")
				if r.Status != "not_checked" || !strings.Contains(r.Error, fmt.Sprintf("line %d:", line)) || !strings.Contains(r.Error, "Missing") {
					t.Fatalf("logical line number changed: %+v", r)
				}
			})
			t.Run("escaped string and source hashes", func(t *testing.T) {
				source := scalar("Init == x = \"a\\rb\"\nNext == UNCHANGED x\nSpec == Init /\\ [][Next]_vars\n\\* property\nP == x = \"a\\rb\"")
				config := "\\* configuration\nSPECIFICATION Spec\nINVARIANT P\n"
				source = strings.ReplaceAll(source, "\n", tc.ending)
				config = strings.ReplaceAll(config, "\n", tc.ending)
				proof := filepath.Join(t.TempDir(), "Check.lean")
				r := model(t, source, config, "-proof", proof)
				if r.Status != "ok" || !r.Complete || r.Proof != proof {
					t.Fatalf("newline-equivalent source failed: %+v", r)
				}
				data, err := os.ReadFile(proof)
				if err != nil {
					t.Fatal(err)
				}
				for _, want := range []string{
					fmt.Sprintf("-- Module SHA-256: %x", sha256.Sum256([]byte(source))),
					fmt.Sprintf("-- Configuration SHA-256: %x", sha256.Sum256([]byte(config))),
					`-- State 0: {"x":"a\rb"}`,
				} {
					if !strings.Contains(string(data), want) {
						t.Fatalf("proof lost original bytes or escaped string data: missing %s", want)
					}
				}
			})
		})
	}
}
