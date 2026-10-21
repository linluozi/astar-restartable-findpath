# Tests

`go test ./...` runs in this directory as-is (the package sources are the patched
ones). The upstream test files are byte-identical to v0.3.0; `newtests_test.go`
is new.

## Per-test results

Measured with Go 1.26.9 on Windows 11, `go test -count=1 -v ./...`.

| test | file | upstream v0.3.0 | patched |
|---|---|---|---|
| `TestPushPop` | `pqueue_test.go` | pass | pass |
| `TestSetAddContains` | `set_test.go` | pass | pass |
| `TestFindPath` (2 subtests) | `astar_test.go` | pass | pass |
| `ExampleFindPath` | `example1_test.go` | pass | pass |
| `ExampleFindPath_maze` | `example2_test.go` | pass | pass |
| `TestFindPathInconsistentHeuristic` | `newtests_test.go` | **FAIL** `cost = 5.1900, want 4.9600` | pass |
| `TestFindPathRandomAdmissibleHeuristics` | `newtests_test.go` | **FAIL** `28 of 3000 random graphs returned a sub-optimal path` | pass |
| `TestFindPathAllocsPerNode` | `newtests_test.go` | **FAIL** `3800 allocations ... want <= 2500` | pass (1897) |
| `TestFindPathLargeInconsistentHeuristic` | `newtests_test.go` | pass | pass |
| `TestFindPathReexpandsImprovedNode` | `newtests_test.go` | pass | pass |
| `TestFindPathMatchesZeroHeuristicOnWeightedGraphs` | `newtests_test.go` | pass | pass |
| `TestFindPathStartEqualsDest` | `newtests_test.go` | pass | pass |
| `TestFindPathMazeLayout` | `newtests_test.go` | pass | pass |
| `TestFindPathManySearchesHaveConstantFootprint` | `newtests_test.go` | pass | pass |
| `TestFindPathDoesNotMutateInputs` | `newtests_test.go` | pass | pass |

Totals: 15 tests/examples pass on the patched tree. On the untouched tree, 12
pass and 3 fail.

## Notes on the two tests that pass on both sides

* `TestFindPathLargeInconsistentHeuristic` — I built this grid instance by hand
  and expected it to trigger the defect; it does not. The heuristic is
  admissible, but this particular layout happens to pop the destination only
  after its cost is final. It is kept as a regression test, not as evidence.
* `TestFindPathReexpandsImprovedNode` — the graph was designed so that a node is
  improved after expansion, and upstream does build a second queue entry for it
  and does discard that entry when it pops. On this instance the discarded entry
  was not on the optimal route, so the returned cost is still optimal. Kept as a
  regression test, not as evidence.

A structural test that asserted "no node is expanded more than twice" was tried
and removed: the unpatched code expands every node at most once, so the
assertion cannot fail on upstream and therefore proves nothing.
