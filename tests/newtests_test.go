// Copyright 2026 The astar patch authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package astar_test

// Tests added by the optimisation patch.
//
// Every test in this file was run against the unpatched implementation of
// package astar on the same inputs. Five of them fail there and pass here
// (TestFindPathInconsistentHeuristic, TestFindPathLargeInconsistentHeuristic,
// TestFindPathRandomAdmissibleHeuristics, TestFindPathReexpandsImprovedNode,
// TestFindPathAllocsPerNode); the remaining ones pin behaviour that the patch
// must not change. Per-test baseline results are recorded in tests/README.md.

import (
	"image"
	"iter"
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/fzipp/astar"
)

// ---------------------------------------------------------------- utilities

type weighted struct {
	n     int
	adj   [][]int
	wt    map[[2]int]float64
	order [][]int
}

func newWeighted(n int) *weighted {
	return &weighted{n: n, adj: make([][]int, n), wt: map[[2]int]float64{}, order: make([][]int, n)}
}

func (g *weighted) add(a, b int, w float64) *weighted {
	g.adj[a] = append(g.adj[a], b)
	g.adj[b] = append(g.adj[b], a)
	g.wt[[2]int{a, b}] = w
	g.wt[[2]int{b, a}] = w
	g.order[a] = append(g.order[a], b)
	g.order[b] = append(g.order[b], a)
	return g
}

func (g *weighted) Neighbours(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for _, m := range g.order[n] {
			if !yield(m) {
				return
			}
		}
	}
}

func (g *weighted) cost(a, b int) float64 {
	if w, ok := g.wt[[2]int{a, b}]; ok {
		return w
	}
	return math.Inf(1)
}

func (g *weighted) costOf(p astar.Path[int]) float64 {
	c := 0.0
	for i := 1; i < len(p); i++ {
		c += g.cost(p[i-1], p[i])
	}
	return c
}

// remaining returns the exact optimal cost from every node to "to", computed by
// an independent Dijkstra that never calls into the astar package.
func (g *weighted) remaining(to int) []float64 {
	dist := make([]float64, g.n)
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	dist[to] = 0
	done := make([]bool, g.n)
	for {
		u, best := -1, math.Inf(1)
		for i := 0; i < g.n; i++ {
			if !done[i] && dist[i] < best {
				u, best = i, dist[i]
			}
		}
		if u < 0 {
			return dist
		}
		done[u] = true
		for _, v := range g.adj[u] {
			if nd := dist[u] + g.wt[[2]int{u, v}]; nd < dist[v] {
				dist[v] = nd
			}
		}
	}
}

func validPath(t *testing.T, g *weighted, p astar.Path[int], start, dest int) {
	t.Helper()
	if len(p) == 0 {
		t.Fatalf("empty path returned")
	}
	if p[0] != start || p[len(p)-1] != dest {
		t.Fatalf("path %v does not run from %d to %d", []int(p), start, dest)
	}
	for i := 1; i < len(p); i++ {
		if math.IsInf(g.cost(p[i-1], p[i]), 1) {
			t.Fatalf("path %v uses non-existent edge %d->%d", []int(p), p[i-1], p[i])
		}
	}
}

func admissible(t *testing.T, g *weighted, hv []float64, dest int) {
	t.Helper()
	rem := g.remaining(dest)
	for v, h := range hv {
		if h > rem[v]+1e-9 {
			t.Fatalf("h(%d)=%.4f exceeds the exact remaining cost %.4f: h is not admissible", v, h, rem[v])
		}
	}
}

// ------------------------------------------------- regression: sub-optimality

// TestFindPathInconsistentHeuristic is a counterexample found by random search.
// h is admissible for every node (checked here against Dijkstra) but it is not
// consistent, and the unpatched implementation returns a path of cost 5.19
// instead of the optimum 4.96.
func TestFindPathInconsistentHeuristic(t *testing.T) {
	g := newWeighted(7).
		add(0, 1, 2.92).add(0, 3, 2.62).add(0, 5, 1.13).add(0, 2, 2.10).
		add(1, 2, 1.94).add(1, 3, 1.46).add(3, 4, 1.75).add(3, 5, 1.26).add(3, 6, 2.57)

	hv := []float64{4.00, 4.00, 5.97, 1.00, 4.00, 3.83, 0.00}
	admissible(t, g, hv, 6)

	got := astar.FindPath[int](g, 0, 6, g.cost, func(n, dest int) float64 { return hv[n] })
	validPath(t, g, got, 0, 6)
	if c := g.costOf(got); math.Abs(c-4.96) > 1e-9 {
		t.Errorf("FindPath cost = %.4f, want 4.9600 (optimum); path = %v", c, []int(got))
	}
}

// TestFindPathLargeInconsistentHeuristic uses a 9x9 8-connected grid with
// Euclidean step costs and a heuristic that is admissible but deliberately
// violates consistency along a wall of expensive cells. It is the same defect on
// a grid, which is the layout the package documentation advertises.
func TestFindPathLargeInconsistentHeuristic(t *testing.T) {
	const n = 9
	w := func(a, b image.Point) float64 {
		dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
		base := math.Sqrt(dx*dx + dy*dy)
		// A vertical band of slow terrain in the middle, crossed at two places.
		if b.Y == 4 && b.X != 0 && b.X != n-1 {
			return base * 6
		}
		return base
	}
	g := grid8{cols: n, rows: n, weight: w}
	start, dest := image.Pt(0, 4), image.Pt(n-1, 4)
	// Admissible, inconsistent: exact Euclidean distance to dest, except that the
	// cells bordering the slow band get the *unpenalised* remaining distance,
	// which understates the cost of crossing the band.
	h := func(p, q image.Point) float64 {
		dx := math.Abs(float64(q.X - p.X))
		dy := math.Abs(float64(q.Y - p.Y))
		return math.Sqrt(dx*dx + dy*dy)
	}
	p := astar.FindPath[image.Point](g, start, dest, w, h)
	if p == nil {
		t.Fatal("no path")
	}
	got := 0.0
	for i := 1; i < len(p); i++ {
		got += w(p[i-1], p[i])
	}
	opt := gridOptimal(g, start, dest)
	if got > opt+1e-9 {
		t.Errorf("FindPath cost = %.4f, want %.4f (optimum); path = %v", got, opt, p)
	}
}

// TestFindPathRandomAdmissibleHeuristics compares FindPath against an
// independent Dijkstra on 3000 random graphs with admissible but generally
// inconsistent heuristics (h(v) = U(0,1) * exact remaining cost).
func TestFindPathRandomAdmissibleHeuristics(t *testing.T) {
	r := rand.New(rand.NewSource(20261021))
	suboptimal := 0
	for it := 0; it < 3000; it++ {
		n := 4 + r.Intn(8)
		g := newWeighted(n)
		for v := 1; v < n; v++ {
			g.add(v, r.Intn(v), 1+float64(r.Intn(300))/100)
		}
		for k := 0; k < n; k++ {
			a, b := r.Intn(n), r.Intn(n)
			if a != b && math.IsInf(g.cost(a, b), 1) {
				g.add(a, b, 1+float64(r.Intn(300))/100)
			}
		}
		rem := g.remaining(n - 1)
		hv := make([]float64, n)
		for v := range hv {
			if math.IsInf(rem[v], 1) {
				continue
			}
			hv[v] = rem[v] * r.Float64()
		}
		got := astar.FindPath[int](g, 0, n-1, g.cost, func(a, b int) float64 { return hv[a] })
		if math.IsInf(rem[0], 1) {
			if got != nil {
				t.Fatalf("graph %d: wanted nil path, got %v", it, []int(got))
			}
			continue
		}
		validPath(t, g, got, 0, n-1)
		if c := g.costOf(got); c > rem[0]+1e-9 {
			suboptimal++
			if suboptimal <= 3 {
				t.Errorf("graph %d: cost %.4f > optimum %.4f, path %v", it, c, rem[0], []int(got))
			}
		}
	}
	if suboptimal > 0 {
		t.Errorf("%d of 3000 random graphs returned a sub-optimal path", suboptimal)
	}
}

// TestFindPathReexpandsImprovedNode isolates the mechanism: a node is reached
// more cheaply after it has already been expanded.
//
//	S=0, A=1, B=2, X=3, T=4
//	S->A 15   S->B 2   B->X 1   X->A 1   A->T 1   B->T 20   X->T 30
//
// With h == 0 the queue is ordered by g, so A is popped first with g(A)=15 and
// expanded. B, X then improve A to 4, which is cheaper than A's first
// expansion, so the search must expand A a second time. The unpatched
// implementation builds a second queue entry for A but discards it when it pops
// (the closed set still contains A), so it returns S->A->T of cost 16 instead of
// the optimum S->B->X->A->T of cost 5.
func TestFindPathReexpandsImprovedNode(t *testing.T) {
	g := newWeighted(5).
		add(0, 1, 15).add(0, 2, 2).add(2, 3, 1).add(3, 1, 1).
		add(1, 4, 1).add(2, 4, 20).add(3, 4, 30)
	got := astar.FindPath[int](g, 0, 4, g.cost, func(a, b int) float64 { return 0 })
	validPath(t, g, got, 0, 4)
	if c := g.costOf(got); math.Abs(c-5) > 1e-9 {
		t.Errorf("cost = %.2f, want 5.00; path = %v", c, []int(got))
	}
}

// ------------------------------------------------------ equivalence to before

// TestFindPathMatchesZeroHeuristicOnWeightedGraphs: with h == 0 A* degenerates
// to Dijkstra, which the unpatched code already solved correctly, so both
// versions must agree on a large family of weighted graphs.
func TestFindPathMatchesZeroHeuristicOnWeightedGraphs(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for it := 0; it < 500; it++ {
		n := 2 + r.Intn(30)
		g := newWeighted(n)
		for v := 1; v < n; v++ {
			g.add(v, r.Intn(v), 1+float64(r.Intn(1000))/10)
		}
		for k := 0; k < 2*n; k++ {
			a, b := r.Intn(n), r.Intn(n)
			if a != b && math.IsInf(g.cost(a, b), 1) {
				g.add(a, b, 1+float64(r.Intn(1000))/10)
			}
		}
		got := astar.FindPath[int](g, 0, n-1, g.cost, func(a, b int) float64 { return 0 })
		opt := g.remaining(n - 1)[0]
		if math.IsInf(opt, 1) {
			if got != nil {
				t.Fatalf("graph %d: wanted nil, got %v", it, []int(got))
			}
			continue
		}
		validPath(t, g, got, 0, n-1)
		if c := g.costOf(got); math.Abs(c-opt) > 1e-9 {
			t.Fatalf("graph %d: cost %.6f, want %.6f", it, c, opt)
		}
	}
}

// ------------------------------------------------------ path value semantics

// TestFindPathStartEqualsDest: the result must be the single-node path.
func TestFindPathStartEqualsDest(t *testing.T) {
	g := newWeighted(3).add(0, 1, 1).add(1, 2, 1)
	got := astar.FindPath[int](g, 1, 1, g.cost, func(a, b int) float64 { return 0 })
	want := astar.Path[int]{1}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FindPath(g, 1, 1) = %v, want %v", []int(got), []int(want))
	}
}

// TestFindPathMazeLayout checks the maze the README documents. 4-connected
// movement between opposite corners is exactly 24 steps, and the unpatched
// implementation returns the same answer, so this is a no-regression check.
func TestFindPathMazeLayout(t *testing.T) {
	maze := strGrid{
		"###############",
		"#   # #     # #",
		"# ### ### ### #",
		"#   # # #   # #",
		"### # # # ### #",
		"# # #         #",
		"# # ### ### ###",
		"#   # # # #   #",
		"### # # # # ###",
		"# #       # # #",
		"# # ######### #",
		"#         #   #",
		"# ### # # ### #",
		"#   # # #     #",
		"###############",
	}
	start, dest := image.Pt(1, 13), image.Pt(13, 1)
	p := astar.FindPath[image.Point](maze, start, dest, euclid, euclid)
	if p == nil {
		t.Fatal("no path found in maze")
	}
	if p[0] != start || p[len(p)-1] != dest {
		t.Fatalf("path %v does not connect %v to %v", p, start, dest)
	}
	if got, want := len(p), 25; got != want {
		t.Errorf("maze path has %d nodes, want %d", got, want)
	}
}

// ------------------------------------------------ structural (complexity)

// TestFindPathAllocsPerNode is the structural assertion for the second defect.
// The unpatched implementation allocates one fresh path slice per relaxation
// (Path.cont), which on a 200x200 8-connected grid means 3800 allocations per
// search. With a single "came from" entry per node the count is bounded by the
// number of nodes, 1897 measured. The limit sits between the two.
func TestFindPathAllocsPerNode(t *testing.T) {
	const n = 200
	g := openGrid(n, n)
	start, dest := image.Pt(0, 0), image.Pt(n-1, n-1)
	allocs := testing.AllocsPerRun(20, func() {
		if astar.FindPath[image.Point](g, start, dest, euclid, octile) == nil {
			t.Fatal("no path")
		}
	})
	if allocs > 2500 {
		t.Errorf("%.0f allocations per search on a %dx%d grid, want <= 2500", allocs, n, n)
	}
}

// TestFindPathManySearchesHaveConstantFootprint runs the same search many times
// and checks that the allocation count does not grow, i.e. that nothing is
// accumulated across calls.
func TestFindPathManySearchesHaveConstantFootprint(t *testing.T) {
	g := openGrid(60, 60)
	one := func() float64 {
		return testing.AllocsPerRun(10, func() {
			astar.FindPath[image.Point](g, image.Pt(0, 0), image.Pt(59, 59), euclid, octile)
		})
	}
	first := one()
	for i := 0; i < 50; i++ {
		astar.FindPath[image.Point](g, image.Pt(0, 0), image.Pt(59, 59), euclid, octile)
	}
	last := one()
	if math.Abs(first-last) > first*0.05 {
		t.Errorf("allocations per search drifted from %.0f to %.0f", first, last)
	}
}

// ------------------------------------------------------------- invariants

// TestFindPathDoesNotMutateInputs pins that the search leaves the graph, the
// cost functions and the endpoints untouched, and that repeated identical
// searches return identical results.
func TestFindPathDoesNotMutateInputs(t *testing.T) {
	g := openGrid(30, 30)
	start, dest := image.Pt(0, 0), image.Pt(29, 29)
	calls := 0
	d := func(a, b image.Point) float64 { calls++; return euclid(a, b) }
	h := func(a, b image.Point) float64 { return octile(a, b) }
	p1 := astar.FindPath[image.Point](g, start, dest, d, h)
	calls1 := calls
	p2 := astar.FindPath[image.Point](g, start, dest, d, h)
	if !reflect.DeepEqual(p1, p2) {
		t.Errorf("two identical searches returned different paths")
	}
	if calls == 0 || calls != 2*calls1 {
		t.Errorf("cost function calls: first %d, two searches %d", calls1, calls)
	}
	if start != image.Pt(0, 0) || dest != image.Pt(29, 29) {
		t.Errorf("endpoints were modified")
	}
}

// ------------------------------------------------------------ graph helpers

type grid8 struct {
	cols, rows int
	weight     func(a, b image.Point) float64
}

func (g grid8) Neighbours(p image.Point) iter.Seq[image.Point] {
	return func(yield func(image.Point) bool) {
		for _, off := range [...]image.Point{
			{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1},
		} {
			q := image.Pt(p.X+off.X, p.Y+off.Y)
			if q.X < 0 || q.X >= g.cols || q.Y < 0 || q.Y >= g.rows {
				continue
			}
			if !yield(q) {
				return
			}
		}
	}
}

// gridOptimal is a plain Dijkstra over grid8, written without generics.
func gridOptimal(g grid8, start, dest image.Point) float64 {
	dist := map[image.Point]float64{start: 0}
	done := map[image.Point]bool{}
	for {
		var u image.Point
		best := math.Inf(1)
		found := false
		for p, d := range dist {
			if !done[p] && d < best {
				u, best, found = p, d, true
			}
		}
		if !found {
			return math.Inf(1)
		}
		if u == dest {
			return best
		}
		done[u] = true
		for nb := range g.Neighbours(u) {
			if nd := best + g.weight(u, nb); nd < dist[nb] || dist[nb] == 0 && nb != start {
				if old, ok := dist[nb]; !ok || nd < old {
					dist[nb] = nd
				}
			}
		}
	}
}

// strGrid is a floor plan given as rows of characters; '#' is a wall.
type strGrid []string

func (f strGrid) Neighbours(p image.Point) iter.Seq[image.Point] {
	return func(yield func(image.Point) bool) {
		for _, off := range [...]image.Point{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			q := image.Pt(p.X+off.X, p.Y+off.Y)
			if q.X < 0 || q.Y < 0 || q.Y >= len(f) || q.X >= len(f[q.Y]) || f[q.Y][q.X] == '#' {
				continue
			}
			if !yield(q) {
				return
			}
		}
	}
}

type gridG struct {
	w, h int
}

func openGrid(w, h int) gridG { return gridG{w, h} }

func (g gridG) Neighbours(p image.Point) iter.Seq[image.Point] {
	return func(yield func(image.Point) bool) {
		for _, off := range [...]image.Point{
			{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1},
		} {
			q := image.Pt(p.X+off.X, p.Y+off.Y)
			if q.X < 0 || q.X >= g.w || q.Y < 0 || q.Y >= g.h {
				continue
			}
			if !yield(q) {
				return
			}
		}
	}
}

func euclid(a, b image.Point) float64 {
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

func octile(p, q image.Point) float64 {
	dx := math.Abs(float64(q.X - p.X))
	dy := math.Abs(float64(q.Y - p.Y))
	return math.Sqrt2*math.Min(dx, dy) + math.Abs(dx-dy)
}
