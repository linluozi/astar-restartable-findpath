// Copyright 2024 Frederik Zipp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package astar

type set[Elem comparable] struct {
	data map[Elem]struct{}
}

// Add adds an element to a set.
func (s *set[Elem]) Add(v Elem) {
	if s.data == nil {
		s.data = make(map[Elem]struct{})
	}
	s.data[v] = struct{}{}
}

// Contains reports whether v is in the set.
func (s *set[Elem]) Contains(v Elem) bool {
	if s.data == nil {
		return false
	}
	_, ok := s.data[v]
	return ok
}
