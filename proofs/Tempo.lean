-- ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
-- Use of this source code is governed by a MIT-style
-- license that can be found in the LICENSE file.

namespace Tempo

universe u v w

/-- A pure deterministic state transition with an explicit observation. -/
structure Model (S : Type u) (E : Type v) (O : Type w) where
  step : S → E → S
  observe : S → O

variable {S T U : Type u} {E : Type v} {O A B : Type w}

def run (m : Model S E O) (s : S) : List E → S
  | [] => s
  | e :: es => run m (m.step s e) es

@[simp] theorem run_nil (m : Model S E O) (s : S) : run m s [] = s := rfl

@[simp] theorem run_cons (m : Model S E O) (s : S) (e : E) (es : List E) :
    run m s (e :: es) = run m (m.step s e) es := rfl

theorem run_append (m : Model S E O) (s : S) (xs ys : List E) :
    run m s (xs ++ ys) = run m (run m s xs) ys := by
  induction xs generalizing s with
  | nil => rfl
  | cons e xs ih => exact ih (m.step s e)

/-- Equality of every future observation, including the current one. -/
def Equivalent (m : Model S E O) (s t : S) : Prop :=
  ∀ es, m.observe (run m s es) = m.observe (run m t es)

theorem equivalent_refl (m : Model S E O) (s : S) : Equivalent m s s :=
  fun _ => rfl

theorem equivalent_symm (m : Model S E O) {s t : S} (h : Equivalent m s t) :
    Equivalent m t s := fun es => (h es).symm

theorem equivalent_trans (m : Model S E O) {s t r : S}
    (h : Equivalent m s t) (k : Equivalent m t r) : Equivalent m s r :=
  fun es => (h es).trans (k es)

theorem equivalent_observe (m : Model S E O) {s t : S} (h : Equivalent m s t) :
    m.observe s = m.observe t := h []

theorem equivalent_step (m : Model S E O) {s t : S}
    (h : Equivalent m s t) (e : E) : Equivalent m (m.step s e) (m.step t e) :=
  fun es => h (e :: es)

/-- Every observation-preserving transition-closed relation preserves behavior. -/
theorem equivalent_of_closed (m : Model S E O) (rel : S → S → Prop)
    (obs : ∀ s t, rel s t → m.observe s = m.observe t)
    (next : ∀ s t, rel s t → ∀ e, rel (m.step s e) (m.step t e))
    {s t : S} (h : rel s t) : Equivalent m s t := by
  intro es
  induction es generalizing s t with
  | nil => exact obs s t h
  | cons e es ih => exact ih (next s t h e)

/-- An exact state mapping preserves observations and every transition. -/
structure Hom (m : Model S E O) (n : Model T E O) where
  map : S → T
  observe_map : ∀ s, n.observe (map s) = m.observe s
  step_map : ∀ s e, map (m.step s e) = n.step (map s) e

def Hom.identity (m : Model S E O) : Hom m m where
  map := id
  observe_map := fun _ => rfl
  step_map := fun _ _ => rfl

def Hom.comp {m : Model S E O} {n : Model T E O} {p : Model U E O}
    (g : Hom n p) (f : Hom m n) : Hom m p where
  map := g.map ∘ f.map
  observe_map := fun s => (g.observe_map (f.map s)).trans (f.observe_map s)
  step_map := fun s e =>
    (congrArg g.map (f.step_map s e)).trans (g.step_map (f.map s) e)

theorem Hom.run_map {m : Model S E O} {n : Model T E O}
    (f : Hom m n) (s : S) (es : List E) :
    f.map (run m s es) = run n (f.map s) es := by
  induction es generalizing s with
  | nil => rfl
  | cons e es ih =>
    exact (ih (m.step s e)).trans (congrArg (fun t => run n t es) (f.step_map s e))

theorem Hom.observes_run {m : Model S E O} {n : Model T E O}
    (f : Hom m n) (s : S) (es : List E) :
    n.observe (run n (f.map s) es) = m.observe (run m s es) := by
  rw [← f.run_map]
  exact f.observe_map _

/-- An exact summary cannot identify behaviorally distinguishable states. -/
theorem Hom.kernel_le_equivalent {m : Model S E O} {n : Model T E O}
    (f : Hom m n) {s t : S} (h : f.map s = f.map t) : Equivalent m s t := by
  intro es
  calc
    m.observe (run m s es) = n.observe (run n (f.map s) es) := (f.observes_run s es).symm
    _ = n.observe (run n (f.map t) es) := congrArg (fun x => n.observe (run n x es)) h
    _ = m.observe (run m t es) := f.observes_run t es

def behaviorSetoid (m : Model S E O) : Setoid S where
  r := Equivalent m
  iseqv := ⟨equivalent_refl m, equivalent_symm m, equivalent_trans m⟩

/-- The behavioral quotient carries well-defined observations and transitions. -/
def quotient (m : Model S E O) : Model (Quotient (behaviorSetoid m)) E O where
  observe := Quotient.lift m.observe (fun _ _ h => equivalent_observe m h)
  step := fun q e => Quotient.liftOn q
    (fun s => Quotient.mk (behaviorSetoid m) (m.step s e))
    (fun _ _ h => Quotient.sound (equivalent_step m h e))

def canonical (m : Model S E O) : Hom m (quotient m) where
  map := Quotient.mk (behaviorSetoid m)
  observe_map := fun _ => rfl
  step_map := fun _ _ => rfl

theorem canonical_exact (m : Model S E O) (s t : S) :
    (canonical m).map s = (canonical m).map t ↔ Equivalent m s t := by
  change Quotient.mk (behaviorSetoid m) s = Quotient.mk (behaviorSetoid m) t ↔ _
  exact ⟨fun h => Quotient.exact h, fun h => Quotient.sound h⟩

def product (left : Model S E A) (right : Model T E B) : Model (S × T) E (A × B) where
  step := fun s e => (left.step s.1 e, right.step s.2 e)
  observe := fun s => (left.observe s.1, right.observe s.2)

theorem run_product (left : Model S E A) (right : Model T E B)
    (s : S × T) (es : List E) :
    run (product left right) s es = (run left s.1 es, run right s.2 es) := by
  induction es generalizing s with
  | nil => rfl
  | cons e es ih => exact ih ((product left right).step s e)

theorem equivalent_product (left : Model S E A) (right : Model T E B)
    (s t : S × T) :
    Equivalent (product left right) s t ↔
      Equivalent left s.1 t.1 ∧ Equivalent right s.2 t.2 := by
  constructor
  · intro h
    constructor
    · intro es
      have he := h es
      rw [run_product, run_product] at he
      exact congrArg Prod.fst he
    · intro es
      have he := h es
      rw [run_product, run_product] at he
      exact congrArg Prod.snd he
  · intro ⟨hl, hr⟩ es
    rw [run_product, run_product]
    exact Prod.ext (hl es) (hr es)

/-- A predicate recurs beyond every finite prefix. -/
def InfinitelyOften (p : Nat → Prop) : Prop :=
  ∀ n, ∃ m, n ≤ m ∧ p m

/-- Removing a finite prefix does not change recurrence. -/
theorem infinitelyOften_shift (k : Nat) (p : Nat → Prop) :
    InfinitelyOften (fun n => p (k + n)) ↔ InfinitelyOften p := by
  constructor
  · intro h n
    obtain ⟨m, hm, hp⟩ := h n
    exact ⟨k + m, Nat.le_trans hm (Nat.le_add_left m k), hp⟩
  · intro h n
    obtain ⟨m, hm, hp⟩ := h (k + n)
    refine ⟨m - k, Nat.le_sub_of_add_le (by simpa only [Nat.add_comm] using hm), ?_⟩
    simpa only [Nat.add_sub_of_le (Nat.le_trans (Nat.le_add_right k n) hm)] using hp

/-- Continuous enablement from some point requires recurrent occurrences. -/
def WeakFair (enabled occurs : Nat → Prop) : Prop :=
  (∃ n, ∀ m, n ≤ m → enabled m) → InfinitelyOften occurs

theorem weakFair_iff (enabled occurs : Nat → Prop) :
    WeakFair enabled occurs ↔
      InfinitelyOften (fun n => ¬enabled n) ∨ InfinitelyOften occurs := by
  classical
  constructor
  · intro h
    by_cases he : ∃ n, ∀ m, n ≤ m → enabled m
    · exact Or.inr (h he)
    · left
      intro n
      exact Classical.byContradiction fun hn =>
        he ⟨n, fun m hm => Classical.byContradiction fun hne => hn ⟨m, hm, hne⟩⟩
  · intro h he
    cases h with
    | inl disabled =>
      obtain ⟨n, hen⟩ := he
      obtain ⟨m, hm, hneg⟩ := disabled n
      exact False.elim (hneg (hen m hm))
    | inr occurs => exact occurs

/-- Weak fairness depends only on the infinite suffix, not its finite stem. -/
theorem weakFair_shift (k : Nat) (enabled occurs : Nat → Prop) :
    WeakFair (fun n => enabled (k + n)) (fun n => occurs (k + n)) ↔
      WeakFair enabled occurs := by
  rw [weakFair_iff, weakFair_iff,
    infinitelyOften_shift k (fun n => ¬enabled n), infinitelyOften_shift k occurs]

theorem infinitelyOften_periodic (k : Nat) (hk : 0 < k) (p : Nat → Prop) :
    InfinitelyOften (fun n => p (n % k)) ↔ ∃ i, i < k ∧ p i := by
  constructor
  · intro h
    obtain ⟨m, _, hp⟩ := h 0
    exact ⟨m % k, Nat.mod_lt m hk, hp⟩
  · intro ⟨i, hi, hp⟩ n
    refine ⟨n * k + i,
      Nat.le_trans (Nat.le_mul_of_pos_right n hk) (Nat.le_add_right _ _), ?_⟩
    simpa only [Nat.mul_add_mod_self_right, Nat.mod_eq_of_lt hi] using hp

/-- A repeated cycle is weakly fair exactly when it contains a disabled phase
or a phase at which the nonstuttering action occurs. -/
theorem weakFair_periodic (k : Nat) (hk : 0 < k) (enabled occurs : Nat → Prop) :
    WeakFair (fun n => enabled (n % k)) (fun n => occurs (n % k)) ↔
      (∃ i, i < k ∧ ¬enabled i) ∨ (∃ i, i < k ∧ occurs i) := by
  rw [weakFair_iff, infinitelyOften_periodic k hk (fun i => ¬enabled i),
    infinitelyOften_periodic k hk occurs]

/-- State after a finite prefix of an infinite supplied event stream. -/
def trace (m : Model S E O) (s : S) (events : Nat → E) : Nat → S
  | 0 => s
  | n + 1 => m.step (trace m s events n) (events n)

theorem Hom.trace_map {m : Model S E O} {n : Model T E O}
    (f : Hom m n) (s : S) (events : Nat → E) (k : Nat) :
    f.map (trace m s events k) = trace n (f.map s) events k := by
  induction k with
  | zero => rfl
  | succ k ih => simp only [trace, f.step_map, ih]

theorem Hom.observes_trace {m : Model S E O} {n : Model T E O}
    (f : Hom m n) (s : S) (events : Nat → E) :
    (fun k => n.observe (trace n (f.map s) events k)) =
      (fun k => m.observe (trace m s events k)) := by
  funext k
  rw [← f.trace_map]
  exact f.observe_map _

theorem Hom.trace_property {m : Model S E O} {n : Model T E O}
    (f : Hom m n) (s : S) (events : Nat → E) (property : (Nat → O) → Prop) :
    property (fun k => n.observe (trace n (f.map s) events k)) ↔
      property (fun k => m.observe (trace m s events k)) := by
  rw [f.observes_trace]

theorem equivalent_trace (m : Model S E O) {s t : S} (h : Equivalent m s t)
    (events : Nat → E) :
    (fun k => m.observe (trace m s events k)) =
      (fun k => m.observe (trace m t events k)) := by
  have related : ∀ k, Equivalent m (trace m s events k) (trace m t events k) := by
    intro k
    induction k with
    | zero => exact h
    | succ k ih => exact equivalent_step m ih (events k)
  funext k
  exact equivalent_observe m (related k)

/-- Every trigger has a response at the same or a later position. -/
def Response (trigger goal : Nat → Prop) : Prop :=
  ∀ n, trigger n → ∃ k, n ≤ k ∧ goal k

/-- The same abstract execution witnesses all invocation obligations. -/
theorem response_of_refinement {Concrete Abstract Identity : Type}
    (admissibleC : Concrete → Prop) (admissibleA : Abstract → Prop)
    (admitC respondC : Concrete → Identity → Nat → Prop)
    (admitA respondA : Abstract → Identity → Nat → Prop)
    (realize : ∀ c, admissibleC c → ∃ a, admissibleA a ∧
      ∀ q n, admitC c q n → ∃ k, admitA a q k ∧
        ∀ l, k ≤ l → respondA a q l → ∃ m, n ≤ m ∧ respondC c q m)
    (live : ∀ a, admissibleA a → ∀ q, Response (admitA a q) (respondA a q)) :
    ∀ c, admissibleC c → ∀ q, Response (admitC c q) (respondC c q) := by
  intro c hc
  obtain ⟨a, ha, transport⟩ := realize c hc
  intro q n hn
  obtain ⟨k, hk, reflect⟩ := transport q n hn
  obtain ⟨l, hkl, hl⟩ := live a ha q k hk
  exact reflect l hkl hl

theorem response_of_weakFair (pending goal enabled occurs : Nat → Prop)
    (fair : WeakFair enabled occurs)
    (keep : ∀ n, pending n → ¬goal n → pending (n + 1))
    (available : ∀ n, pending n → ¬goal n → enabled n)
    (serve : ∀ n, pending n → ¬goal n → occurs n → goal (n + 1)) :
    Response pending goal := by
  classical
  intro start hp
  apply Classical.byContradiction
  intro absent
  have noGoal : ∀ n, start ≤ n → ¬goal n := fun n hn hg => absent ⟨n, hn, hg⟩
  have stays : ∀ d, pending (start + d) := by
    intro d
    induction d with
    | zero => simpa using hp
    | succ d ih => exact keep (start + d) ih (noGoal _ (by omega))
  have allPending : ∀ n, start ≤ n → pending n := by
    intro n hn
    obtain ⟨d, rfl⟩ := Nat.exists_eq_add_of_le hn
    exact stays d
  obtain ⟨n, hn, action⟩ := fair ⟨start, fun n hn =>
    available n (allPending n hn) (noGoal n hn)⟩ start
  exact noGoal (n + 1) (by omega) (serve n (allPending n hn) (noGoal n hn) action)

/-- A natural sequence with no increases eventually attains its least value. -/
theorem rank_stabilizes (rank : Nat → Nat) (step : ∀ n, rank (n + 1) ≤ rank n) :
    ∃ n, ∀ m, n ≤ m → rank m = rank n := by
  classical
  have least : ∀ v, (∃ n, rank n = v) → ∃ n, ∀ m, rank n ≤ rank m := by
    intro v
    induction v using Nat.strongRecOn with
    | ind v ih =>
      rintro ⟨n, hn⟩
      by_cases bound : ∀ m, rank n ≤ rank m
      · exact ⟨n, bound⟩
      · have smaller : ∃ m, rank m < rank n := by
          apply Classical.byContradiction
          intro none
          apply bound
          intro m
          have notSmaller : ¬rank m < rank n := fun hm => none ⟨m, hm⟩
          omega
        obtain ⟨m, hm⟩ := smaller
        exact ih (rank m) (by omega) ⟨m, rfl⟩
  obtain ⟨n, minimal⟩ := least (rank 0) ⟨0, rfl⟩
  refine ⟨n, ?_⟩
  intro m order
  apply Nat.le_antisymm ?_ (minimal m)
  obtain ⟨d, rfl⟩ := Nat.exists_eq_add_of_le order
  clear order
  induction d with
  | zero => exact Nat.le_refl _
  | succ d ih => exact Nat.le_trans (step (n + d)) ih

/-- Boolean traversal checks every state without building nested quantified proofs. -/
theorem of_checkFinite {n : Nat} (P : Fin n → Prop) [DecidablePred P]
    (checked : (List.finRange n).all (fun s => decide (P s)) = true) : ∀ s, P s :=
  fun s => of_decide_eq_true ((List.all_eq_true.mp checked) s (List.mem_finRange s))

/-- Exact finite graph data. Action numbers name weak-fairness obligations. -/
structure Graph (n : Nat) where
  edges : Fin n → List (Fin n × List Nat)
  enabled : Fin n → List Nat

def Graph.Next {n : Nat} (g : Graph n) (s t : Fin n) : Prop :=
  ∃ actions, (t, actions) ∈ g.edges s

def Graph.Occurs {n : Nat} (g : Graph n) (j : Nat) (s t : Fin n) : Prop :=
  ∃ actions, (t, actions) ∈ g.edges s ∧ j ∈ actions

def Graph.Path {n : Nat} (g : Graph n) (path : Nat → Fin n) : Prop :=
  ∀ k, g.Next (path k) (path (k + 1))

def Graph.Fair {n : Nat} (g : Graph n) (path : Nat → Fin n) : Prop :=
  ∀ j, WeakFair (fun k => j ∈ g.enabled (path k))
    (fun k => g.Occurs j (path k) (path (k + 1)))

/-- An avoiding region and a weak-fair task blocking each stable rank. -/
structure Certificate (n : Nat) where
  seed : Fin n → Bool
  goal : Fin n → Bool
  region : Fin n → Bool
  rank : Fin n → Nat
  blocker : Nat → Nat

def Certificate.Valid {n : Nat} (c : Certificate n) (g : Graph n) : Prop :=
  ∀ s,
    (c.seed s = true → c.goal s = false → c.region s = true) ∧
    (c.region s = true →
      c.blocker (c.rank s) ∈ g.enabled s ∧
      ∀ edge ∈ g.edges s, c.goal edge.1 = false →
        c.region edge.1 = true ∧ c.rank edge.1 ≤ c.rank s ∧
        (c.rank edge.1 = c.rank s → c.blocker (c.rank s) ∉ edge.2))

/-- Kernel-checkable finite certificates exclude every fair avoiding suffix. -/
theorem Certificate.sound {n : Nat} (c : Certificate n) (g : Graph n)
    (valid : c.Valid g) (path : Nat → Fin n) (steps : g.Path path) (fair : g.Fair path) :
    Response (fun k => c.seed (path k) = true) (fun k => c.goal (path k) = true) := by
  classical
  intro start triggered
  apply Classical.byContradiction
  intro absent
  have noGoal : ∀ d, c.goal (path (start + d)) = false := by
    intro d
    cases h : c.goal (path (start + d))
    · rfl
    · exact False.elim (absent ⟨start + d, by omega, h⟩)
  have inside : ∀ d, c.region (path (start + d)) = true := by
    intro d
    induction d with
    | zero => exact (valid (path start)).1 triggered (noGoal 0)
    | succ d ih =>
      obtain ⟨actions, edge⟩ := steps (start + d)
      exact ((valid _).2 ih).2 _ edge (noGoal (d + 1)) |>.1
  have decreasing : ∀ d, c.rank (path (start + (d + 1))) ≤ c.rank (path (start + d)) := by
    intro d
    obtain ⟨actions, edge⟩ := steps (start + d)
    exact (((valid _).2 (inside d)).2 _ edge (noGoal (d + 1))).2.1
  obtain ⟨offset, stable⟩ := rank_stabilizes (fun d => c.rank (path (start + d))) decreasing
  let j := c.blocker (c.rank (path (start + offset)))
  have enabled : ∃ k, ∀ l, k ≤ l → j ∈ g.enabled (path l) := by
    refine ⟨start + offset, ?_⟩
    intro l hl
    have h := ((valid _).2 (inside (l - start))).1
    have eqRank := stable (l - start) (by omega)
    simp only [Nat.add_sub_of_le (show start ≤ l by omega)] at h eqRank
    rw [eqRank] at h
    exact h
  obtain ⟨l, hl, actions, edge, occurs⟩ := fair j enabled (start + offset)
  have zeroRank := stable (l - start) (by omega)
  have nextRank := stable (l + 1 - start) (by omega)
  have inRegion := inside (l - start)
  have away := noGoal (l + 1 - start)
  simp only [Nat.add_sub_of_le (show start ≤ l by omega)] at zeroRank inRegion
  simp only [Nat.add_sub_of_le (show start ≤ l + 1 by omega)] at nextRank away
  have blocked := (((valid _).2 inRegion).2 (path (l + 1), actions) edge away).2.2
    (nextRank.trans zeroRank.symm)
  exact blocked (by simpa [j, zeroRank] using occurs)

end Tempo
