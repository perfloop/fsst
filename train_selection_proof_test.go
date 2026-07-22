package fsst

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

const selectionProofTableSHA256 = "682a257952d7305bdc87f63e5eba2b0304d3128683d98f7c409de0b7f9be298d"

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
		if got, want := serialized.Len(), 534; got != want {
			t.Fatalf("serialized table length is %d, want %d", got, want)
		}
		tableHash := sha256.Sum256(serialized.Bytes())
		if got := hex.EncodeToString(tableHash[:]); got != selectionProofTableSHA256 {
			t.Fatalf("serialized table hash is %s, want %s", got, selectionProofTableSHA256)
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
