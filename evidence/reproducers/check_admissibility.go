//go:build ignore

package main

// Check whether the smallest counterexample found by the random search really
// needs an inconsistent heuristic, and whether the node that gets mis-closed
// is reached more cheaply later.

import (
	"fmt"
	"math"

	"github.com/fzipp/astar"
)

func main() {
	const n = 7
	edges := [][3]float64{
		{0, 1, 2.92}, {0, 3, 2.62}, {0, 5, 1.13}, {0, 2, 2.10},
		{1, 2, 1.94}, {1, 3, 1.46}, {3, 4, 1.75}, {3, 5, 1.26}, {3, 6, 2.57},
	}
	h := []float64{4.00, 4.00, 5.97, 1.00, 4.00, 3.83, 0.00}

	w := make([][]float64, n)
	for i := range w {
		w[i] = make([]float64, n)
		for j := range w[i] {
			w[i][j] = math.Inf(1)
		}
	}
	for _, e := range edges {
		a, b, c := int(e[0]), int(e[1]), e[2]
		w[a][b], w[b][a] = c, c
	}

	// exact remaining distance to destination 6, by Bellman-Ford from 6
	rem := make([]float64, n)
	for i := range rem {
		rem[i] = math.Inf(1)
	}
	rem[6] = 0
	for it := 0; it < n; it++ {
		for _, e := range edges {
			a, b, c := int(e[0]), int(e[1]), e[2]
			if rem[b]+c < rem[a] {
				rem[a] = rem[b] + c
			}
			if rem[a]+c < rem[b] {
				rem[b] = rem[a] + c
			}
		}
	}
	fmt.Printf("exact remaining cost to 6: %v\n", rem)
	fmt.Printf("h chosen                : %v\n", h)
	for v := 0; v < n; v++ {
		if h[v] > rem[v]+1e-9 {
			fmt.Printf("  h[%d]=%.2f > rem=%.2f  => h is NOT admissible\n", v, h[v], rem[v])
		}
	}
	// consistency: h[u] <= w(u,v) + h[v] for all edges
	for _, e := range edges {
		a, b, c := int(e[0]), int(e[1]), e[2]
		if h[a] > c+h[b]+1e-9 {
			fmt.Printf("  inconsistent: h[%d]=%.2f > w(%d,%d)+h[%d] = %.2f+%.2f\n", a, h[a], a, b, b, c, h[b])
		}
		if h[b] > c+h[a]+1e-9 {
			fmt.Printf("  inconsistent: h[%d]=%.2f > w+ h[%d] = %.2f+%.2f\n", b, h[b], a, c, h[a])
		}
	}
	_ = astar.Path[int]{}
}
