-- ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
-- Use of this source code is governed by a MIT-style
-- license that can be found in the LICENSE file.

import Tempo
import Lean

open Lean Elab Command in
run_cmd do
  let env ← getEnv
  let required : Array Name := #[
    `Tempo.run_append, `Tempo.equivalent_observe, `Tempo.equivalent_step,
    `Tempo.equivalent_of_closed, `Tempo.Hom.run_map,
    `Tempo.Hom.observes_run, `Tempo.Hom.kernel_le_equivalent,
    `Tempo.canonical_exact, `Tempo.run_product, `Tempo.equivalent_product,
    `Tempo.weakFair_iff, `Tempo.infinitelyOften_shift, `Tempo.weakFair_shift,
    `Tempo.infinitelyOften_periodic, `Tempo.weakFair_periodic,
    `Tempo.Hom.trace_map, `Tempo.Hom.observes_trace, `Tempo.Hom.trace_property,
    `Tempo.equivalent_trace, `Tempo.response_of_refinement,
    `Tempo.response_of_weakFair, `Tempo.rank_stabilizes, `Tempo.of_checkFinite,
    `Tempo.Certificate.sound]
  for name in required do
    unless (env.find? name).any (fun info => info.isTheorem) do
      throwError "Required theorem is missing: {name}"
  let allowed : Array Name := #[`propext, `Classical.choice, `Quot.sound]
  let mut checked : Nat := 0
  for (name, info) in env.constants.toList do
    if (env.getModuleIdxFor? name).any (fun idx => env.header.moduleNames[idx]! == `Tempo) then
      if let .axiomInfo _ := info then
        throwError "Local axioms are not permitted: {name}"
      if info.isTheorem then
        let axioms ← collectAxioms name
        for axiomName in axioms do
          unless allowed.contains axiomName do
            throwError "Unapproved axiom {axiomName} in {name}"
        logInfo m!"{name}: {axioms}"
        checked := checked + 1
  if checked == 0 then
    throwError "No temporal theorems were checked"
  logInfo m!"Checked {checked} temporal theorems"
