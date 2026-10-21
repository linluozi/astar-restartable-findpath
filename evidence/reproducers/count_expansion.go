//go:build ignore

package main

// Probe: quantify the cost of materialising a full Path value at every
// hedge push in fzipp/astar's FindPath.
//
// Measures, for one search:
//   - pops / pushes
//   - total nodes copied by Path.cont (sum of len(p)+1 per push)
//   - peak live heap (runtime.MemStats.HeapInuse after the search, GC forced)
//   - wall time
//
// Graph family: uniform 8-connected grid with Manhattan heuristic scaled by
// sqrt(2) (admissible, consistent) -- the layout the README documents.

import (
	"fmt"
	"image"
	"iter"
	"math"
	"runtime"
	"time"

	"github.com/fzipp/astar"
)

type grid struct {
	w, h int
	wall func(x, y int) bool
}

var diagOffsets = [...]image.Point{
	{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1},
}

func (g grid) Neighbours(p image.Point) iter.Seq[image.Point] {
	return func(yield func(image.Point) bool) {
		for _, off := range diagOffsets {
			q := image.Pt(p.X+off.X, p.Y+off.Y)
			if q.X < 0 || q.X >= g.w || q.Y < 0 || q.Y >= g.h {
				continue
			}
			if g.wall(q.X, q.Y) {
				continue
			}
			if !yield(q) {
				return
			}
		}
	}
}

func cost(a, b image.Point) float64 {
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

func heur(p, q image.Point) float64 {
	dx := math.Abs(float64(q.X - p.X))
	dy := math.Abs(float64(q.Y - p.Y))
	return math.Sqrt2*math.Min(dx, dy) + math.Abs(dx-dy)
}

func uniform(w, h int) grid {
	return grid{w: w, h: h, wall: func(x, y int) bool { return false }}
}

// rooms: 20x20 cells, one-cell wall between them, every room has exactly one
// door on a side (deterministic pseudo-random which side/offset).
func rooms(cellW, cellH, n int) grid {
	w, h := n*(cellW+1)+1, n*(cellH+1)+1
	walls := make([][]bool, h)
	for y := range walls {
		walls[y] = make([]bool, w)
	}
	for x := 0; x < w; x++ {
		walls[0][x], walls[h-1][x] = true, true
	}
	for y := 0; y < h; y++ {
		walls[y][0], walls[y][w-1] = true, true
	}
	seed := uint32(12345)
	rnd := func(mod int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % mod
	}
	for cy := 0; cy < n; cy++ {
		for cx := 0; cx < n; cx++ {
			x0 := cx*(cellW+1) + 1
			y0 := cy*(cellH+1) + 1
			// interior wall between this room and the east/south neighbour
			if cx < n-1 {
				wallX := x0 + cellW
				doorY := y0 + rnd(cellH)
				for y := y0 - 1; y <= y0+cellH; y++ {
					if y == doorY {
						continue
					}
					if y >= 0 && y < h {
						walls[y][wallX] = true
					}
				}
			}
			if cy < n-1 {
				wallY := y0 + cellH
				doorX := x0 + rnd(cellW)
				for x := x0 - 1; x <= x0+cellW; x++ {
					if x == doorX {
						continue
					}
					if x >= 0 && x < w {
						walls[wallY][x] = true
					}
				}
			}
		}
	}
	return grid{w: w, h: h, wall: func(x, y int) bool { return walls[y][x] }}
}

func measure(name string, g grid, start, dest image.Point) {
	runtime.GC()
	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	t0 := time.Now()
	p := astar.FindPath[image.Point](g, start, dest, cost, heur)
	el := time.Since(t0)

	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)

	pathCost := 0.0
	if p != nil {
		pathCost = p.Cost(cost)
	}
	_ = pathCost
	fmt.Printf("%-22s pathLen=%d cost=%.4f time=%8.3fs heapInuse=%6.2fMiB totalAlloc=%8.2fMiB\n",
		name, len(p), pathCost, el.Seconds(),
		float64(after.HeapInuse)/1024/1024,
		float64(after.TotalAlloc-before.TotalAlloc)/1024/1024)
}

func main() {
	cases := []struct {
		name       string
		g          grid
		start, dst image.Point
	}{
		{"uniform-200x200", uniform(200, 200), image.Pt(0, 0), image.Pt(199, 199)},
		{"uniform-500x500", uniform(500, 500), image.Pt(0, 0), image.Pt(499, 499)},
		{"rooms-8x8", rooms(20, 20, 8), image.Pt(2, 2), image.Pt(8*21-2, 8*21-2)},
	}
	for _, c := range cases {
		measure(c.name, c.g, c.start, c.dst)
	}
	for _, c := range cases {
		report(c.name, c.g, c.start, c.dst)
	}
}

// report prints the instrumented counters for the same searches.
//
// The counters (astar.ResetStats / astar.Stats{Pops,Pushes,Copied}) are not part
// of the published package: they were added to a private copy of upstream, as
// described in README.md. Against the pristine upstream this prints a note
// instead.
func report(name string, g grid, start, dest image.Point) {
	_ = astar.FindPath[image.Point](g, start, dest, cost, heur)
	fmt.Printf("%-22s (counters require an instrumented build)\n", name)
}
