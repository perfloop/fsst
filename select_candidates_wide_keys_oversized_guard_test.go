package fsst

import "testing"

func BenchmarkSelectCandidatesWideKeysOversized(b *testing.B) {
	benchmarkSelectCandidatesWideKeys(b, maxCandidateSymbols*8)
}
