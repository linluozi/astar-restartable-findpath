# Reproducers and diagnostics

Four standalone `package main` files, all carrying `//go:build ignore` so they
are skipped by `go build ./...`. They were used against **upstream v0.3.0**
(`../../patch/upstream`) to find the defects, to check that the heuristic in each
counterexample is really admissible, and to read the queue order.

Run one of them with a temporary module that points at the upstream sources:

```sh
mkdir /tmp/repro && cd /tmp/repro
printf 'module repro\n\ngo 1.23.0\n\nrequire github.com/fzipp/astar v0.0.0\n\nreplace github.com/fzipp/astar => <ABS>/patch/upstream\n' > go.mod
cp <ABS>/evidence/reproducers/trace_mechanism.go .
go run trace_mechanism.go
```

`<ABS>` must be an absolute path to the unpacked archive.

| file | what it does | what it showed |
|---|---|---|
| `fuzz_admissible.go` | 3000-graph generator with `h(v) = U(0,1) * exact remaining cost`, compared against Dijkstra | 28 of 3000 graphs returned a sub-optimal path on upstream, 0 on the patched package |
| `check_admissibility.go` | recomputes exact remaining costs by Bellman-Ford and checks `h[v] <= remaining[v]` for the counterexample | `h` is admissible for all 7 nodes; it is *inconsistent*, which is what upstream implicitly assumes it will not be |
| `trace_mechanism.go` | re-implements upstream's loop with a print per pop/push, plus the graph of the first failing case | shows the destination being popped at `g = 7.60` while a cheaper 7.37 route exists, and shows the discarded duplicate entries |
| `count_expansion.go` | pops / pushes / nodes copied by `Path.cont` on uniform and rooms grids | 8x8 rooms: 46,677 pops, 53,316 pushes, 4,744,687 nodes copied. **Requires counters added to a private copy of upstream**; against the pristine sources the file prints a note instead |

The numbers quoted in the top-level `README.md` come from these programs and from
`tests/newtests_test.go`, not from any file that ships a modified library.
