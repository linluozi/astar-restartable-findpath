// Copyright 2013 Frederik Zipp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package astar implements the A* search algorithm for finding least-cost paths.
package astar

import (
	"container/heap"
	"iter"
)

// The Graph interface is the minimal interface a graph data structure
// must satisfy to be suitable for the A* algorithm.
type Graph[Node any] interface {
	// Neighbours returns the neighbour nodes of node n in the graph.
	Neighbours(n Node) iter.Seq[Node]
}

// A CostFunc is a function that returns a cost for the transition
// from node a to node b.
type CostFunc[Node any] func(a, b Node) float64

// A Path is a sequence of nodes in a graph.
type Path[Node any] []Node

// newPath creates a new path with one start node. More nodes can be
// added with append().
func newPath[Node any](start Node) Path[Node] {
	return []Node{start}
}

// last returns the last node of path p. It is not removed from the path.
func (p Path[Node]) last() Node {
	return p[len(p)-1]
}

// cont creates a new path, which is a continuation of path p with the
// additional node n.
func (p Path[Node]) cont(n Node) Path[Node] {
	cp := make([]Node, len(p), len(p)+1)
	copy(cp, p)
	cp = append(cp, n)
	return cp
}

// Cost calculates the total cost of path p by applying the cost function d
// to all path segments and returning the sum.
func (p Path[Node]) Cost(d CostFunc[Node]) (c float64) {
	for i := 1; i < len(p); i++ {
		c += d(p[i-1], p[i])
	}
	return c
}

// FindPath finds the least-cost path between start and dest in graph g
// using the cost function d and the cost heuristic function h.
// Returns nil if no path was found.
//
// h must be admissible: it must not overestimate the real cost from a node to
// dest, and h(dest, dest) must be 0. In exchange, h does not have to be
// consistent.
//
// The search keeps at most one open-set entry per node, keyed by the node's
// best known f value, and materialises a Path only once, for the destination.
// Each node stores a single "came from" reference instead of one complete path
// copy per relaxation, and a node whose cost improves after it has been
// expanded is expanded again rather than ignored.
func FindPath[Node comparable](g Graph[Node], start, dest Node, d, h CostFunc[Node]) Path[Node] {
	open := newOpenSet[Node]()
	heap.Init(open)
	open.setKey(start, h(start, dest))
	heap.Push(open, start)

	var cameFrom cameFromMap[Node]
	best := map[Node]float64{start: 0}

	for open.Len() > 0 {
		_, n := open.PopEntry()
		if n == dest {
			return reconstruct(cameFrom, start, dest)
		}

		gn := best[n]
		for nb := range g.Neighbours(n) {
			// c is the cost of the best known route to nb via n.
			c := gn + d(n, nb)
			old, known := best[nb]
			if known && !(c < old) {
				continue
			}
			best[nb] = c
			cameFrom.set(nb, n)
			open.requeue(nb, c+h(nb, dest))
		}
	}

	// No path found
	return nil
}

// reconstruct builds the path from start to dest by walking the "came from"
// chain backwards.
func reconstruct[Node comparable](cf cameFromMap[Node], start, dest Node) Path[Node] {
	if start == dest {
		return newPath(start)
	}
	rev := []Node{dest}
	n := dest
	for {
		p, ok := cf.get(n)
		if !ok {
			return nil
		}
		rev = append(rev, p)
		if p == start {
			break
		}
		n = p
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}
