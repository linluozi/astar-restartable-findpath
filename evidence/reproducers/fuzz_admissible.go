//go:build ignore

package main

// Exhaustive search for inputs on which fzipp/astar v0.3.0 FindPath returns a
// sub-optimal path. Anything found here is a *real* counterexample: it does not
// depend on my reading of the source, only on the comparison against an
// independent Dijkstra.

import (
	"fmt"
	"image"
	"iter"
	"math"
	"math/rand"

	"github.com/fzipp/astar"
)

type g1 struct {
	n     int // nodes are 0..n-1
	adj   [][]int
	wt    []map[int]float64
	order [][]int
}

func (g *g1) Neighbours(v int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for _, m := range g.order[v] {
			if !yield(m) {
				return
			}
		}
	}
}

func (g *g1) cost(a, b int) float64 {
	if c, ok := g.wt[a][b]; ok {
		return c
	}
	return math.Inf(1)
}

// dijkstraOpt returns the optimal cost from 0 to n-1.
func (g *g1) dijkstraOpt() float64 {
	dist := make([]float64, g.n)
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	dist[0] = 0
	done := make([]bool, g.n)
	for {
		u, best := -1, math.Inf(1)
		for i := 0; i < g.n; i++ {
			if !done[i] && dist[i] < best {
				u, best = i, dist[i]
			}
		}
		if u < 0 {
			return math.Inf(1)
		}
		if u == g.n-1 {
			return dist[u]
		}
		done[u] = true
		for _, v := range g.adj[u] {
			if nd := dist[u] + g.wt[u][v]; nd < dist[v] {
				dist[v] = nd
			}
		}
	}
}

// remainingToDest returns, for every node v, the exact optimal cost v -> n-1.
func (g *g1) remainingToDest() []float64 {
	rev := &g1{n: g.n, adj: g.adj, wt: g.wt, order: g.order}
	dist := make([]float64, g.n)
	for i := range dist {
		dist[i] = math.Inf(1)
	}
	dist[g.n-1] = 0
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
		for _, v := range rev.adj[u] {
			if nd := dist[u] + rev.wt[u][v]; nd < dist[v] {
				dist[v] = nd
			}
		}
	}
}

func (g *g1) pathCost(p astar.Path[int]) float64 {
	t := 0.0
	for i := 1; i < len(p); i++ {
		t += g.cost(p[i-1], p[i])
	}
	return t
}

// randomGraph builds a connected random graph on n nodes with 0 = start.
func randomGraph(r *rand.Rand, n int) *g1 {
	g := &g1{n: n, adj: make([][]int, n), wt: make([]map[int]float64, n), order: make([][]int, n)}
	for i := range g.wt {
		g.wt[i] = map[int]float64{}
	}
	add := func(a, b int, w float64) {
		if _, ok := g.wt[a][b]; ok {
			return
		}
		g.adj[a] = append(g.adj[a], b)
		g.adj[b] = append(g.adj[b], a)
		g.wt[a][b] = w
		g.wt[b][a] = w
	}
	for i := 1; i < n; i++ { // random spanning tree keeps it connected
		add(i, r.Intn(i), 1+float64(r.Intn(200))/100)
	}
	extra := n
	for i := 0; i < extra; i++ {
		add(r.Intn(n), r.Intn(n), 1+float64(r.Intn(200))/100)
	}
	for i := range g.order {
		o := append([]int(nil), g.adj[i]...)
		r.Shuffle(len(o), func(a, b int) { o[a], o[b] = o[b], o[a] })
		g.order[i] = o
	}
	return g
}

func dump(g *g1, opt, got float64, p astar.Path[int], h map[int]float64, n int) []string {
	out := []string{fmt.Sprintf("  n=%d opt=%.4f got=%.4f path=%v", n, opt, got, []int(p))}
	hlist := "  h = "
	for v := 0; v < n; v++ {
		hlist += fmt.Sprintf("%.2f ", h[v])
	}
	out = append(out, hlist)
	for i := 0; i < n; i++ {
		for _, m := range g.adj[i] {
			if i < m {
				out = append(out, fmt.Sprintf("  %d -- %d : %.2f", i, m, g.wt[i][m]))
			}
		}
	}
	return out
}

func main() {
	r := rand.New(rand.NewSource(7))
	bad, total := 0, 0
	var first *g1
	var firstP astar.Path[int]
	var firstEdges []string
	for iter := 0; iter < 20000; iter++ {
		n := 4 + r.Intn(6)
		g := randomGraph(r, n)
		opt := g.dijkstraOpt()
		if math.IsInf(opt, 1) {
			continue
		}
		// heuristic values: integer 0..maxW so admissible-ness is easy to violate;
		// we accept the case only if h is *admissible* (h[v] <= true remaining cost
		// from v to dest) computed by an independent Dijkstra run from dest.
		h := make(map[int]float64)
		rem := g.remainingToDest()
		admissible := true
		for v := 0; v < n; v++ {
			hv := float64(r.Intn(int(opt*2) + 1))
			if hv > rem[v]+1e-9 {
				hv = rem[v]
			}
			if hv > rem[v]+1e-9 {
				admissible = false
			}
			h[v] = hv
		}
		if !admissible {
			continue
		}
		total++
		p := astar.FindPath[int](g, 0, n-1, g.cost, func(a, b int) float64 { return h[a] })
		if p == nil {
			continue
		}
		if c := g.pathCost(p); c > opt+1e-9 {
			bad++
			if len(p) <= 3 && len(firstEdges) == 0 {
				firstEdges = dump(g, opt, c, p, h, n)
			}
			if first == nil {
				first, firstP = g, p
				fmt.Printf("first counterexample: n=%d opt=%.3f got=%.3f path=%v h=", n, opt, c, []int(p))
				for v := 0; v < n; v++ {
					fmt.Printf("%.2f ", h[v])
				}
				fmt.Println()
				for i := 0; i < n; i++ {
					for _, m := range g.adj[i] {
						if i < m {
							fmt.Printf("  %d--%d %.2f\n", i, m, g.wt[i][m])
						}
					}
				}
			}
		}
	}
	fmt.Printf("admissible-h random graphs: %d, sub-optimal results: %d\n", total, bad)
	_ = firstP
	_ = image.Pt
	fmt.Println("smallest counterexample found:")
	for _, l := range firstEdges {
		fmt.Println(l)
	}
}
