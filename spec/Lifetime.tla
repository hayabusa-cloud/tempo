\* ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
\* Use of this source code is governed by a MIT-style
\* license that can be found in the LICENSE file.

------------------------------ MODULE Lifetime ------------------------------
EXTENDS Naturals

CONSTANTS Deadline, MaxTime
ASSUME Deadline \in Nat
       /\ MaxTime \in Nat
       /\ Deadline < MaxTime

VARIABLES request, loan
vars == <<request, loan>>
Events == {"cancel", "complete", "settle", "return", "move", "tick", "invalid"}

Init ==
    request = [cancelled |-> FALSE, complete |-> FALSE,
                   expired |-> FALSE, rejected |-> FALSE,
                   now |-> 0, origin |-> 0, deadline |-> Deadline, owner |-> 0]
    /\ loan = [settled |-> FALSE, returned |-> FALSE,
                rejected |-> FALSE, custodian |-> 0]

RequestStep(r, e) ==
    CASE e = "cancel" -> [r EXCEPT !.cancelled = TRUE]
      [] e = "complete" -> [r EXCEPT !.complete = TRUE]
      [] e = "move" -> [r EXCEPT !.owner = 1]
      [] e = "tick" ->
           LET t == IF r.now < MaxTime THEN r.now + 1 ELSE r.now
           IN [r EXCEPT !.now = t,
                        !.expired = @ \/ (~r.complete /\ t > r.deadline)]
      [] e \in {"settle", "return"} -> r
      [] OTHER -> [r EXCEPT !.rejected = TRUE]

LoanStep(l, e) ==
    CASE e = "settle" -> [l EXCEPT !.settled = TRUE]
      [] e = "return" -> IF l.settled THEN [l EXCEPT !.returned = TRUE]
                                      ELSE [l EXCEPT !.rejected = TRUE]
      [] e \in {"cancel", "complete", "move", "tick"} -> l
      [] OTHER -> [l EXCEPT !.rejected = TRUE]

Apply(e) == request' = RequestStep(request, e)
            /\ loan' = LoanStep(loan, e)
Next == \E e \in Events : Apply(e)

SafetySpec == Init /\ [][Next]_vars
FairSpec == SafetySpec
            /\ WF_vars(Apply("complete"))
            /\ WF_vars(Apply("settle"))
            /\ WF_vars(Apply("return"))

TypeOK ==
    request \in [cancelled : BOOLEAN, complete : BOOLEAN,
                     expired : BOOLEAN, rejected : BOOLEAN,
                     now : 0..MaxTime, origin : {0}, deadline : {Deadline},
                     owner : {0, 1}]
    /\ loan \in [settled : BOOLEAN, returned : BOOLEAN,
                  rejected : BOOLEAN, custodian : {0}]
ReturnedAfterSettlement == loan.returned => loan.settled
OriginalObligation == request.origin = 0 /\ request.deadline = Deadline
OriginalCustodian == loan.custodian = 0
EventuallyComplete == <>request.complete
EventuallyReturned == <>loan.returned

\* A counterexample control that omits target settlement.
UnsafeReturn == UNCHANGED request
                /\ loan' = [loan EXCEPT !.returned = TRUE]
UnsafeSpec == Init /\ [][Next \/ UnsafeReturn]_vars
=============================================================================
