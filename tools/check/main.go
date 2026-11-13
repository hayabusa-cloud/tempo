// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Command tempo-check exhaustively checks a constructive finite TLA+ fragment.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
)

func readSource(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	const limit = 1 << 20
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return "", err
	}
	if len(b) > limit {
		return "", fmt.Errorf("source size exceeds %d bytes", limit)
	}
	return string(b), nil
}

func run(args []string) result {
	bad := func(err error) result { return result{Status: "not_checked", Error: err.Error()} }
	flags := flag.NewFlagSet("tempo-check", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	config := flags.String("config", "", "configuration file")
	proof := flags.String("proof", "", "write a Lean proof of successful finite-graph checks")
	states := flags.Int("max-states", 50000, "maximum reachable states")
	transitions := flags.Int("max-transitions", 1000000, "maximum distinct edges, including stuttering")
	values := flags.Int("max-values", 10000, "maximum finite enumeration, alternatives, or value nodes")
	steps := flags.Int("max-steps", 10000000, "maximum expression evaluations and action expansions")
	if err := flags.Parse(args); err != nil {
		return bad(err)
	}
	if flags.NArg() != 1 || *config == "" {
		return bad(fmt.Errorf("usage: tempo-check -config model.cfg [-proof check.lean] [-max-states N] [-max-transitions N] [-max-values N] [-max-steps N] model.tla"))
	}
	if *states <= 0 || *transitions <= 0 || *values <= 0 || *steps <= 0 {
		return bad(fmt.Errorf("checking bounds must be positive"))
	}
	source, err := readSource(flags.Arg(0))
	if err != nil {
		return bad(err)
	}
	m, err := parseModule(source)
	if err != nil {
		return bad(err)
	}
	moduleHash := sha256.Sum256([]byte(source))
	source, err = readSource(*config)
	if err != nil {
		return bad(err)
	}
	c, err := parseConfig(source)
	if err != nil {
		return bad(err)
	}
	r := check(m, c, bounds{states: *states, transitions: *transitions, values: *values, steps: *steps}, *proof != "")
	if r.Status == "ok" && *proof != "" {
		configHash := sha256.Sum256([]byte(source))
		if err := r.proof.write(*proof, moduleHash, configHash); err != nil {
			r.Status, r.Error = "not_checked", "proof output: "+err.Error()
		} else {
			r.Proof = *proof
		}
	}
	return r
}

func main() {
	r := run(os.Args[1:])
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	switch r.Status {
	case "ok":
	case "violation":
		os.Exit(1)
	default:
		os.Exit(2)
	}
}
