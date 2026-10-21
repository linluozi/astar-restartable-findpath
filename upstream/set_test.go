// Copyright 2024 Frederik Zipp. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package astar

import "testing"

func TestSetAddContains(t *testing.T) {
	tests := []struct {
		added        []int
		notContained []int
	}{
		{
			added:        nil,
			notContained: []int{1, 2, 3},
		},
		{
			added:        []int{1, 2, 3, 4},
			notContained: []int{-2, -1, 0, 5, 6},
		},
		{
			added:        []int{4, 4, 5, 9, 9, 10},
			notContained: []int{0, 1, 2, 3, 6, 7, 8, 11, 12},
		},
	}
	for _, tt := range tests {
		var s set[int]
		for _, v := range tt.added {
			s.Add(v)
		}
		for _, v := range tt.added {
			if !s.Contains(v) {
				t.Errorf("Set of added %v should contain %v", tt.added, v)
			}
		}
		for _, v := range tt.notContained {
			if s.Contains(v) {
				t.Errorf("Set of added %v should not contain %v", tt.added, v)
			}
		}
	}
}
