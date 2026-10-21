package bench

import (
	"image"
	"testing"

	"github.com/fzipp/astar"
)

func bench(c Case) func(*testing.B) {
	return func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			p := astar.FindPath[image.Point](c.Graph, c.Start, c.Dest, Unit, c.H)
			if p == nil {
				b.Fatal("no path")
			}
		}
	}
}

func BenchmarkFindPath(b *testing.B) {
	for _, c := range Cases() {
		b.Run(c.Name, bench(c))
	}
}
