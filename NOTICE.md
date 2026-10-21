# NOTICE

This work is derived from [`fzipp/astar`](https://github.com/fzipp/astar),
version 0.3.0, commit `2b743c63c1bc1e722d5067b7f58c154940b8b8c0`
(2024-08-14), which is Copyright 2013–2024 Frederik Zipp and distributed under
the BSD 3-Clause licence. The original licence text is included unmodified in
`LICENSE`.

Modifications (2026-10-21):

| file | status |
|---|---|
| `astar.go` | modified: `FindPath` rewritten; `Graph`, `CostFunc`, `Path`, `Path.Cost`, `Path.cont`, `Path.last`, `newPath` and the package comment kept |
| `openset.go` | added |
| `camefrom.go` | added |
| `pqueue.go` | unchanged, still compiled, no longer used by `FindPath` |
| `set.go` | unchanged, still compiled, no longer used by `FindPath` |
| `go.mod` | unchanged |
| `README.md`, `CHANGELOG.md` | upstream copies kept in `patch/upstream/`; this directory's `README.md` is new |

The upstream copyright headers at the top of `astar.go`, `pqueue.go` and
`set.go` are kept verbatim. `pqueue.go` additionally carries the Go Authors'
copyright notice, which is also kept verbatim.

New files added by this work are marked
`Copyright 2026 The astar patch authors` and are released under the same
BSD 3-Clause terms as the original.

The measurement harness (`bench/*`, `verify.py`, `evidence/*`) is new work, not
derived from upstream, and carries no upstream copyright. It is included under
the same licence for convenience.

Upstream project page: <https://github.com/fzipp/astar>
Upstream documentation: <https://pkg.go.dev/github.com/fzipp/astar>
