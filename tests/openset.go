// Copyright 2026 The astar patch authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package astar

import "container/heap"

// An openSet is the A* open set: a binary heap of nodes ordered by their f
// value, holding at most one entry per node.
//
// The heap stores node values, not heap items, and each node's f value and
// position are kept in per-node maps. That lets the key of a queued node be
// lowered in place with heap.Fix, instead of pushing a duplicate entry that has
// to be discarded on pop. A node is in exactly one of three states:
//
//   - absent: no entry in the heap (never queued, or popped and not improved)
//   - queued: exactly one entry in the heap, at nodes[pos[n]]
//   - popped: no entry in the heap, but it has been returned by PopEntry
//
// A popped node stays eligible for requeueing: that is what makes
// re-expansion possible without keeping a separate closed set.
type openSet[Node comparable] struct {
	nodes []Node
	pos   map[Node]int
	key   map[Node]float64
}

func newOpenSet[Node comparable]() *openSet[Node] {
	return &openSet[Node]{
		pos: make(map[Node]int),
		key: make(map[Node]float64),
	}
}

func (s *openSet[Node]) Len() int { return len(s.nodes) }

func (s *openSet[Node]) Less(i, j int) bool {
	// heap.Pop returns the minimum, and A* expands the smallest f first.
	return s.key[s.nodes[i]] < s.key[s.nodes[j]]
}

func (s *openSet[Node]) Swap(i, j int) {
	s.nodes[i], s.nodes[j] = s.nodes[j], s.nodes[i]
	s.pos[s.nodes[i]] = i
	s.pos[s.nodes[j]] = j
}

// Push implements heap.Interface. It queues node n with the key set by setKey.
func (s *openSet[Node]) Push(x any) {
	n := x.(Node)
	s.pos[n] = len(s.nodes)
	s.nodes = append(s.nodes, n)
}

// Pop implements heap.Interface. Callers use PopEntry, which also returns the
// key.
func (s *openSet[Node]) Pop() any {
	i := len(s.nodes) - 1
	n := s.nodes[i]
	s.nodes = s.nodes[:i]
	delete(s.pos, n)
	return n
}

// setKey records the f value of node n. It must be called before pushing n.
func (s *openSet[Node]) setKey(n Node, f float64) { s.key[n] = f }

// inQueue reports whether n currently has an entry in the heap.
func (s *openSet[Node]) inQueue(n Node) bool {
	_, ok := s.pos[n]
	return ok
}

// requeue applies an improved f value for node n: if n is in the heap its key
// is lowered in place, otherwise n is pushed again. This is the only place
// where the open set grows, so it grows by at most one entry per node.
func (s *openSet[Node]) requeue(n Node, f float64) {
	if i, ok := s.pos[n]; ok {
		if f < s.key[n] {
			s.key[n] = f
			heap.Fix(s, i)
		}
		return
	}
	s.key[n] = f
	heap.Push(s, n)
}

// PopEntry removes and returns the node with the smallest f value together
// with that value.
func (s *openSet[Node]) PopEntry() (f float64, n Node) {
	n = heap.Pop(s).(Node)
	return s.key[n], n
}
