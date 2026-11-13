\* ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
\* Use of this source code is governed by a MIT-style
\* license that can be found in the LICENSE file.

---- MODULE Service ----
EXTENDS Naturals
VARIABLES phase, owner
vars == <<phase, owner>>

Init == phase = 0 /\ owner = 0
Admit == phase = 0 /\ phase' = 1 /\ UNCHANGED owner
ServeAt(w) == phase = 1 /\ owner = w /\ phase' = 2 /\ UNCHANGED owner
Serve == ServeAt(0) \/ ServeAt(1)
Retire == phase = 2 /\ phase' = 0 /\ UNCHANGED owner
Move == phase = 1 /\ owner' = 1 - owner /\ UNCHANGED phase
Next == Admit \/ Serve \/ Retire \/ Move

Spec == Init /\ [][Next]_vars
FairSpec == Spec /\ WF_vars(Admit) /\ WF_vars(Serve) /\ WF_vars(Retire)
OwnerSpec == Spec /\ WF_vars(Admit) /\ WF_vars(ServeAt(0)) /\ WF_vars(ServeAt(1)) /\ WF_vars(Retire)

TypeOK == phase \in 0..2 /\ owner \in 0..1
Response == [](phase = 1 => <>(phase = 2))
Recurring == []<>(phase = 2)
====
