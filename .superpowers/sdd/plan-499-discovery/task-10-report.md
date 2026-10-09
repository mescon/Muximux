# Task 10 report
Status: DONE. Added missingSince tracking (MarkMissing, MissingSince, RenameTrackedKey, RecordSeen returns recovery, ForgetTrackedKey clears), Poller.resolveFailed, and poller helper noteResolve (log on transitions only). Per-key state is pruned each tick (pruneTrackedState, Service.pruneMissing). Ported the stale test as TestIssue499_AbsentContainerRemovedOnlyBySyncOfAutoImported with real assertions.
Tests: race suite passes, discovery coverage 93.5%, new funcs 83-100%, lint 0 issues, gofmt clean.

## Fix round 1
noteResolve now clears resolveFailed on the not-found path, so an error that returns after an outage logs again (test added). The ported issue499 test gained add and update mode cases, run for 6 ticks (past the grace period), asserting the absent auto-imported app is kept.
