// Copyright 2026 The astar patch authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package astar

// cameFromMap is the "came from" table of the search: it maps a node to its
// predecessor on the best known route from the start node.
//
// It is a plain map rather than a struct wrapping a slice, because node types
// are arbitrary comparable values; the map is the only representation that
// works for all of them without an index space.
type cameFromMap[Node comparable] map[Node]Node

func (cf *cameFromMap[Node]) set(n, from Node) {
	if *cf == nil {
		*cf = make(cameFromMap[Node])
	}
	(*cf)[n] = from
}

func (cf cameFromMap[Node]) get(n Node) (Node, bool) {
	from, ok := cf[n]
	return from, ok
}
