//go:build windows

package bench

// A single-search memory probe.
//
// The benchmark in b_test.go reports B/op (bytes *allocated*, including
// garbage). That is not the same question as "how much memory does the process
// hold". This probe answers the second question, and it reports two numbers
// because neither one is sufficient on its own:
//
//   peak_ws      PROCESS_MEMORY_COUNTERS.PeakWorkingSetSize, read after the
//                search. This is the kernel's own high-water mark for the
//                whole process, so it is exact and it cannot be fooled by a
//                later garbage collection. It also includes the ~5 MB Go
//                runtime and test binary that every arm pays identically;
//                the runner subtracts a `PROBE_CASE=none` baseline to get the
//                incremental footprint of the search.
//
//   heap_peak    MemStats.HeapAlloc sampled from inside the cost function
//                every sampleEvery calls, max over the search. Sampling is
//                approximate (it can miss the true peak between samples) and
//                it reads MemStats with a stop-the-world pause, so it is a
//                secondary number. Its value is that it isolates heap from
//                binary/runtime pages.
//
// One process = one search on purpose: a peak is a maximum over the process
// lifetime, so a second search in the same process would be hidden underneath
// the first one's peak.

import (
	"image"
	"iter"
	"os"
	"runtime"
	"syscall"
	"testing"
	"unsafe"

	"github.com/fzipp/astar"
)

// processMemoryCounters mirrors PROCESS_MEMORY_COUNTERS from psapi.h.
type processMemoryCounters struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
}

var procGetProcessMemoryInfo = syscall.NewLazyDLL("psapi.dll").NewProc("GetProcessMemoryInfo")

func peakWorkingSet() (uint64, error) {
	var c processMemoryCounters
	c.cb = uint32(unsafe.Sizeof(c))
	h, err := syscall.GetCurrentProcess()
	if err != nil {
		return 0, err
	}
	ok, _, callErr := procGetProcessMemoryInfo.Call(
		uintptr(h), uintptr(unsafe.Pointer(&c)), uintptr(c.cb))
	if ok == 0 {
		return 0, callErr
	}
	return uint64(c.peakWorkingSetSize), nil
}

const sampleEvery = 64

// countingGraph counts node expansions, i.e. calls to Neighbours. It is a
// black-box counter: it wraps the graph, not the implementation, so it works
// identically against either arm and cannot be fooled by internal bookkeeping.
type countingGraph struct {
	g          Grid
	expansions *int
}

func (cg countingGraph) Neighbours(p image.Point) iter.Seq[image.Point] {
	*cg.expansions++
	return cg.g.Neighbours(p)
}

func TestMemProbe(t *testing.T) {
	name := os.Getenv("PROBE_CASE")
	if name == "" {
		t.Skip("PROBE_CASE is not set")
	}
	// A wrong layout would silently report a field from the middle of the
	// struct, so pin it: DWORD + DWORD + 8 * SIZE_T.
	if got, want := unsafe.Sizeof(processMemoryCounters{}), uintptr(72); got != want {
		t.Fatalf("PROCESS_MEMORY_COUNTERS layout is %d bytes, want %d", got, want)
	}

	cost, nodes, calls, expansions := -1.0, -1, 0, 0
	var heapPeak uint64
	var samples int

	if name != "none" {
		found := false
		for _, c := range Cases() {
			if c.Name != name {
				continue
			}
			found = true
			// Same step cost as Unit; only instrumented.
			p := astar.FindPath(countingGraph{c.Graph, &expansions}, c.Start, c.Dest,
				sampledCost(&calls, &heapPeak, &samples, Unit), c.H)
			if p == nil {
				t.Fatalf("case %s: no path found", name)
			}
			cost = p.Cost(Unit)
			nodes = len(p)
		}
		if !found {
			t.Fatalf("unknown case %q", name)
		}
	}

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	ws, err := peakWorkingSet()
	if err != nil {
		t.Fatalf("GetProcessMemoryInfo: %v", err)
	}
	if heapPeak == 0 && name != "none" {
		heapPeak = ms.HeapAlloc
		samples = 1
	}
	// One machine-readable line; the runner parses only this prefix.
	t.Logf("PROBE case=%s cost=%.6f nodes=%d peak_ws=%d heap_peak=%d samples=%d expansions=%d d_calls=%d total_alloc=%d mallocs=%d",
		name, cost, nodes, ws, heapPeak, samples, expansions, calls, ms.TotalAlloc, ms.Mallocs)
}

// sampledCost returns d wrapped so that every sampleEvery calls it records the
// current HeapAlloc, keeping the maximum. Both arms get the same wrapper.
func sampledCost(calls *int, peak *uint64, samples *int, d astar.CostFunc[image.Point]) astar.CostFunc[image.Point] {
	return func(a, b image.Point) float64 {
		*calls++
		if *calls%sampleEvery == 0 {
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			if ms.HeapAlloc > *peak {
				*peak = ms.HeapAlloc
			}
			*samples++
		}
		return d(a, b)
	}
}
