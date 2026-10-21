//go:build ignore

package main

// Replays the generator of TestFindPathRandomAdmissibleHeuristics against the
// instrumented upstream implementation. For every graph it records whether the
// result was sub-optimal, and prints the pop trace of the first few failures so
// the mechanism can be read off directly instead of guessed.

import (
	"errors"
	"fmt"
	"iter"
	"math"
	"math/rand"

	"github.com/fzipp/astar"
)

var errStop = errors.New("stop")

type graph struct {
	n     int
	order [][]int
	wt    map[[2]int]float64
}

func newGraph(n int) *graph {
	return &graph{n: n, order: make([][]int, n), wt: map[[2]int]float64{}}
}

func (g *graph) add(a, b int, w float64) {
	if _, ok := g.wt[[2]int{a, b}]; ok {
		return
	}
	g.order[a] = append(g.order[a], b)
	g.order[b] = append(g.order[b], a)
	g.wt[[2]int{a, b}] = w
	g.wt[[2]int{b, a}] = w
}

func (g *graph) Neighbours(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for _, m := range g.order[n] {
			if !yield(m) {
				return
			}
		}
	}
}

func (g *graph) cost(a, b int) float64 {
	if w, ok := g.wt[[2]int{a, b}]; ok {
		return w
	}
	return math.Inf(1)
}

func (g *graph) costOf(p astar.Path[int]) float64 {
	c := 0.0
	for i := 1; i < len(p); i++ {
		c += g.cost(p[i-1], p[i])
	}
	return c
}

func (g *graph) remaining(to int) []float64 {
	d := make([]float64, g.n)
	for i := range d {
		d[i] = math.Inf(1)
	}
	d[to] = 0
	done := make([]bool, g.n)
	for {
		u, best := -1, math.Inf(1)
		for i := 0; i < g.n; i++ {
			if !done[i] && d[i] < best {
				u, best = i, d[i]
			}
		}
		if u < 0 {
			return d
		}
		done[u] = true
		for _, v := range g.order[u] {
			if nd := d[u] + g.wt[[2]int{u, v}]; nd < d[v] {
				d[v] = nd
			}
		}
	}
}

func main() {
	r := rand.New(rand.NewSource(20261021))
	fail, total := 0, 0
	shown := 0
	for it := 0; it < 3000; it++ {
		n := 4 + r.Intn(8)
		g := newGraph(n)
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
			if !math.IsInf(rem[v], 1) {
				hv[v] = rem[v] * r.Float64()
			}
		}
		if math.IsInf(rem[0], 1) {
			continue
		}
		total++
		p := astar.FindPath[int](g, 0, n-1, g.cost, func(a, b int) float64 { return hv[a] })
		if p == nil {
			fmt.Printf("graph %d: nil path but optimum %.3f\n", it, rem[0])
			continue
		}
		if c := g.costOf(p); c > rem[0]+1e-9 {
			fail++
			if shown < 2 {
				shown++
				fmt.Printf("\n=== graph %d: n=%d opt=%.4f got=%.4f path=%v\n", it, n, rem[0], c, []int(p))
				for v := 0; v < n; v++ {
					fmt.Printf("   h[%d]=%.4f rem=%.4f\n", v, hv[v], rem[v])
				}
				for i := 0; i < n; i++ {
					for _, j := range g.order[i] {
						if i < j {
							fmt.Printf("   %d -- %d : %.2f\n", i, j, g.wt[[2]int{i, j}])
						}
					}
				}
				fmt.Println("   --- queue trace ---")
				traceGraph(g, hv, n-1)
			}
		}
	}
	fmt.Printf("\nsub-optimal: %d / %d\n", fail, total)
}

// traceGraph re-implements FindPath with tracing. It is only used to explain the
// mechanism; the result above comes from the real package.
func traceGraph(g *graph, hv []float64, dest int) {
	type entry struct {
		path []int
		f    float64
	}
	var q []entry
	push := func(e entry) {
		q = append(q, e)
		// max-heap by f, mirroring the upstream priority queue
		i := len(q) - 1
		for i > 0 {
			p := (i - 1) / 2
			if q[p].f < q[i].f {
				q[p], q[i] = q[i], q[p]
				i = p
				continue
			}
			break
		}
	}
	pop := func() entry {
		top := q[0]
		q[0] = q[len(q)-1]
		q = q[:len(q)-1]
		i := 0
		for {
			l, rr := 2*i+1, 2*i+2
			m := i
			if l < len(q) && q[l].f > q[m].f {
				m = l
			}
			if rr < len(q) && q[rr].f > q[m].f {
				m = rr
			}
			if m == i {
				break
			}
			q[i], q[m] = q[m], q[i]
			i = m
		}
		return top
	}
	cost := func(p []int) float64 {
		c := 0.0
		for i := 1; i < len(p); i++ {
			c += g.cost(p[i-1], p[i])
		}
		return c
	}
	closed := map[int]bool{}
	push(entry{[]int{0}, hv[0]})
	for len(q) > 0 {
		e := pop()
		n := e.path[len(e.path)-1]
		gn := cost(e.path)
		fmt.Printf("   pop n=%d g=%.4f f=%.4f closed=%v\n", n, gn, e.f, closed[n])
		if closed[n] {
			fmt.Println("     -> discarded (already closed)")
			continue
		}
		if n == dest {
			fmt.Printf("     -> RETURN cost %.4f\n", gn)
			return
		}
		closed[n] = true
		for _, nb := range g.order[n] {
			np := append(append([]int{}, e.path...), nb)
			f := cost(np) + hv[nb]
			fmt.Printf("     push n=%d g=%.4f f=%.4f via %v\n", nb, cost(np), f, np)
			push(entry{np, f})
		}
	}
}
