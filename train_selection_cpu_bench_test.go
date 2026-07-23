//go:build linux

package fsst

import (
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"
)

func processCPUTime(b *testing.B) time.Duration {
	b.Helper()
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err != nil {
		b.Fatalf("get process CPU time: %v", err)
	}
	return time.Duration(usage.Utime.Sec)*time.Second +
		time.Duration(usage.Utime.Usec)*time.Microsecond +
		time.Duration(usage.Stime.Sec)*time.Second +
		time.Duration(usage.Stime.Usec)*time.Microsecond
}

// BenchmarkTrainSelectionCPU measures public Train on the same bounded corpus
// used by the table-output guard. The custom metric measures process CPU spent
// inside Train only, so host descheduling does not dominate the component-cost
// comparison; the standard benchmark metrics remain co-emitted.
func BenchmarkTrainSelectionCPU(b *testing.B) {
	if got := runtime.GOMAXPROCS(0); got != 1 {
		b.Fatalf("GOMAXPROCS = %d, want 1", got)
	}
	data, err := os.ReadFile("testdata/art_of_war.txt")
	if err != nil {
		b.Fatal(err)
	}
	inputs := [][]byte{data}

	b.ReportAllocs()
	var (
		cpuTotal time.Duration
		result   *Table
	)
	for b.Loop() {
		start := processCPUTime(b)
		result = Train(inputs)
		elapsed := processCPUTime(b) - start
		if elapsed < 0 {
			b.Fatal("process CPU time moved backwards")
		}
		cpuTotal += elapsed
	}
	if result == nil || result.nSymbols == 0 {
		b.Fatal("Train did not produce a populated table")
	}
	b.ReportMetric(float64(cpuTotal)/float64(b.N), "cpu-ns/op")
}
