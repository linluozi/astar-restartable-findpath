package bench

// Shared workloads. Every governor in this directory imports this file and
// calls astar.FindPath; which implementation that resolves to is decided by the
// replace directive in go.mod, never by the code below.

import (
	"image"
	"iter"
	"math"

	"github.com/fzipp/astar"
)

var diag = [...]image.Point{
	{0, -1}, {1, -1}, {1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1},
}

// Grid is an 8-connected, obstacle-only grid.
type Grid struct {
	W, H int
	Wall func(x, y int) bool
}

func (g Grid) Neighbours(p image.Point) iter.Seq[image.Point] {
	return func(yield func(image.Point) bool) {
		for _, off := range diag {
			q := image.Pt(p.X+off.X, p.Y+off.Y)
			if q.X < 0 || q.X >= g.W || q.Y < 0 || q.Y >= g.H || g.Wall(q.X, q.Y) {
				continue
			}
			if !yield(q) {
				return
			}
		}
	}
}

// Unit is the Euclidean step cost; it equals the true one-step distance, so it
// is admissible.
func Unit(a, b image.Point) float64 {
	dx, dy := float64(b.X-a.X), float64(b.Y-a.Y)
	return math.Sqrt(dx*dx + dy*dy)
}

// Octile is the standard admissible heuristic for 8-connected unit grids.
func Octile(p, q image.Point) float64 {
	dx := math.Abs(float64(q.X - p.X))
	dy := math.Abs(float64(q.Y - p.Y))
	return math.Sqrt2*math.Min(dx, dy) + math.Abs(dx-dy)
}

// Zero makes the search behave as Dijkstra; useful as an attribution arm.
func Zero(a, b image.Point) float64 { return 0 }

func Open(w, h int) Grid {
	return Grid{w, h, func(x, y int) bool { return false }}
}

// Rooms builds a k x k arrangement of cell x cell rooms separated by walls with
// exactly one door per wall segment (deterministic).
func Rooms(cell, k int) Grid {
	const step = 21
	w, h := k*step+1, k*step+1
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
	seed := uint32(2024)
	rnd := func(mod int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>16) % mod
	}
	for cy := 0; cy < k; cy++ {
		for cx := 0; cx < k; cx++ {
			x0, y0 := cx*step+1, cy*step+1
			if cx < k-1 {
				wx, dy := x0+cell, y0+rnd(cell)
				for y := y0; y < y0+cell; y++ {
					if y != dy {
						walls[y][wx] = true
					}
				}
			}
			if cy < k-1 {
				wy, dx := y0+cell, x0+rnd(cell)
				for x := x0; x < x0+cell; x++ {
					if x != dx {
						walls[wy][x] = true
					}
				}
			}
		}
	}
	return Grid{w, h, func(x, y int) bool { return walls[y][x] }}
}

// Maze is a recursive-backtracker maze on a (2n+1) grid.
func Maze(n int, seed uint32) Grid {
	w, h := 2*n+1, 2*n+1
	walls := make([][]bool, h)
	for y := range walls {
		walls[y] = make([]bool, w)
		for x := range walls[y] {
			walls[y][x] = true
		}
	}
	rnd := func() uint32 {
		seed ^= seed << 13
		seed ^= seed >> 17
		seed ^= seed << 5
		return seed
	}
	type pt struct{ x, y int }
	stack := []pt{{1, 1}}
	walls[1][1] = false
	for len(stack) > 0 {
		c := stack[len(stack)-1]
		var cand []pt
		for _, d := range []pt{{0, -2}, {2, 0}, {0, 2}, {-2, 0}} {
			nx, ny := c.x+d.x, c.y+d.y
			if nx > 0 && ny > 0 && nx < w-1 && ny < h-1 && walls[ny][nx] {
				cand = append(cand, pt{nx, ny})
			}
		}
		if len(cand) == 0 {
			stack = stack[:len(stack)-1]
			continue
		}
		n := cand[rnd()%uint32(len(cand))]
		walls[(c.y+n.y)/2][(c.x+n.x)/2] = false
		walls[n.y][n.x] = false
		stack = append(stack, n)
	}
	return Grid{w, h, func(x, y int) bool { return walls[y][x] }}
}

// Case is one workload.
type Case struct {
	Name  string
	Graph Grid
	Start image.Point
	Dest  image.Point
	H     astar.CostFunc[image.Point]
}

// Cases lists the benchmark workloads in increasing graph size.
func Cases() []Case {
	m := Maze(20, 7)
	return []Case{
		{"open-100x100", Open(100, 100), image.Pt(0, 0), image.Pt(99, 99), Octile},
		{"open-300x300", Open(300, 300), image.Pt(0, 0), image.Pt(299, 299), Octile},
		{"open-600x600", Open(600, 600), image.Pt(0, 0), image.Pt(599, 599), Octile},
		{"rooms-8x8", Rooms(20, 8), image.Pt(2, 2), image.Pt(8*21-2, 8*21-2), Octile},
		{"rooms-12x12", Rooms(20, 12), image.Pt(2, 2), image.Pt(12*21-2, 12*21-2), Octile},
		{"maze-41x41", m, image.Pt(1, 1), image.Pt(39, 39), Octile},
	}
}
