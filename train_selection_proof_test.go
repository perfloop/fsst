package fsst

import (
	"bytes"
	"os"
	"testing"
)

// trainSelectionProofCorpus keeps the proof at Train's public boundary, so the
// benchmark receives candidate states produced by the production counting and
// merging pipeline rather than by a manufactured selector fixture.
func trainSelectionProofCorpus(tb testing.TB) []byte {
	tb.Helper()
	data, err := os.ReadFile("testdata/art_of_war.txt")
	if err != nil {
		tb.Fatalf("read training corpus: %v", err)
	}
	return data
}

func TestTrainSelectionCorpusDeterminismAndRoundTrip(t *testing.T) {
	data := trainSelectionProofCorpus(t)
	inputs := [][]byte{data}

	var wantTable []byte
	for range 4 {
		table := Train(inputs)

		var serialized bytes.Buffer
		if _, err := table.WriteTo(&serialized); err != nil {
			t.Fatalf("serialize trained table: %v", err)
		}
		if wantTable == nil {
			wantTable = bytes.Clone(serialized.Bytes())
		} else if !bytes.Equal(wantTable, serialized.Bytes()) {
			t.Fatal("training produced a different serialized table for the same corpus")
		}

		compressed := table.EncodeAll(data)
		if got := table.DecodeAll(compressed); !bytes.Equal(got, data) {
			t.Fatal("trained table did not round-trip the corpus")
		}

		var restored Table
		if _, err := restored.ReadFrom(bytes.NewReader(serialized.Bytes())); err != nil {
			t.Fatalf("deserialize trained table: %v", err)
		}
		if got := restored.EncodeAll(data); !bytes.Equal(got, compressed) {
			t.Fatal("deserialized table changed corpus compression")
		}
	}
}

func BenchmarkTrainSelectionCorpus(b *testing.B) {
	data := trainSelectionProofCorpus(b)
	inputs := [][]byte{data}

	b.ReportAllocs()
	for b.Loop() {
		_ = Train(inputs)
	}
}
