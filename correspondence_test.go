// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

//go:build tempo_verify

package tempo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

var lifetimeEvents = []struct {
	value event
	name  string
}{
	{cancel, "cancel"},
	{complete, "complete"},
	{settle, "settle"},
	{returnLoan, "return"},
	{move, "move"},
	{tick, "tick"},
	{invalid, "invalid"},
}

func tlaBool(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

func requestRecord(r request) string {
	return fmt.Sprintf("[cancelled |-> %s, complete |-> %s, expired |-> %s, rejected |-> %s, now |-> %d, origin |-> %d, deadline |-> %d, owner |-> %d]",
		tlaBool(r.Cancelled), tlaBool(r.Complete), tlaBool(r.Expired), tlaBool(r.Rejected), r.Now, r.Origin, r.Deadline, r.Owner)
}

func loanRecord(l loan) string {
	return fmt.Sprintf("[settled |-> %s, returned |-> %s, rejected |-> %s, custodian |-> %d]",
		tlaBool(l.Settled), tlaBool(l.Returned), tlaBool(l.Rejected), l.Custodian)
}

// Each assertion compares a Go successor with the original TLA+ operator at
// literal inputs. Only initial states need exploration to check the equations.
func stepChecks[R interface {
	comparable
	Step(event) R
}](t *testing.T, source *strings.Builder, operator string, initial R, encode func(R) string) []string {
	t.Helper()
	seen := map[R]bool{initial: true}
	queue := []R{initial}
	var names []string
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		input := encode(current)
		for _, e := range lifetimeEvents {
			next := current.Step(e.value)
			name := fmt.Sprintf("Go%s%d", operator, len(names))
			fmt.Fprintf(source, "%s == %s(%s, %q) = %s\n", name, operator, input, e.name, encode(next))
			names = append(names, name)
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	t.Logf("%s: %d reachable Go states, %d transition equations", operator, len(queue), len(names))
	return names
}

func TestLifetimeCorrespondence(t *testing.T) {
	module, err := os.ReadFile("spec/Lifetime.tla")
	if err != nil {
		t.Fatal(err)
	}
	end := strings.LastIndex(string(module), "\n====")
	if end < 0 {
		t.Fatal("Lifetime module terminator missing")
	}
	var source strings.Builder
	source.Write(module[:end])
	source.WriteString("\nGoSpec == Init /\\ [][FALSE]_vars\n")
	r, l := request{Origin: origin, Deadline: deadline}, loan{}
	fmt.Fprintf(&source, "GoInitial == request = %s /\\ loan = %s\n", requestRecord(r), loanRecord(l))
	var events []string
	for _, e := range lifetimeEvents {
		events = append(events, fmt.Sprintf("%q", e.name))
	}
	fmt.Fprintf(&source, "GoEvents == Events = {%s}\n", strings.Join(events, ", "))
	checks := []string{"GoInitial", "GoEvents"}
	checks = append(checks, stepChecks(t, &source, "RequestStep", r, requestRecord)...)
	checks = append(checks, stepChecks(t, &source, "LoanStep", l, loanRecord)...)
	source.WriteString("====\n")
	config := fmt.Sprintf("CONSTANTS\nDeadline = %d\nMaxTime = %d\nSPECIFICATION GoSpec\nINVARIANTS %s\n",
		deadline, maxTime, strings.Join(checks, " "))

	dir := t.TempDir()
	modelPath, configPath := filepath.Join(dir, "Lifetime.tla"), filepath.Join(dir, "Go.cfg")
	if err := os.WriteFile(modelPath, []byte(source.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithTimeout(t.Context(), 60*time.Second)
	defer stop()
	checker := filepath.Join(dir, "check")
	build := exec.CommandContext(ctx, "go", "build", "-buildvcs=false", "-race", "-o", checker, ".")
	build.Dir = "tools/check"
	build.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build checker: %v\n%s", err, output)
	}
	command := exec.CommandContext(ctx, checker, "-config", configPath, modelPath)
	command.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Go/TLA+ correspondence: %v\n%s", err, output)
	}
	var result struct {
		Status   string
		Complete bool
		States   int
		Checked  []string
	}
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatalf("checker result: %v\n%s", err, output)
	}
	if result.Status != "ok" || !result.Complete || result.States != 1 || !slices.Equal(result.Checked, checks) {
		t.Fatalf("incomplete Go/TLA+ correspondence:\n%s", output)
	}
}
