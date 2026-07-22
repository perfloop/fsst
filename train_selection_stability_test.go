package fsst

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

// TestTrainArtOfWarSelectionStable locks the serialized table produced by a
// representative corpus whose training path repeatedly ranks candidates.
func TestTrainArtOfWarSelectionStable(t *testing.T) {
	data, err := os.ReadFile("testdata/art_of_war.txt")
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}

	table := Train([][]byte{data})
	serialized, err := table.MarshalBinary()
	if err != nil {
		t.Fatalf("serialize table: %v", err)
	}

	digest := sha256.Sum256(serialized)
	const wantDigest = "682a257952d7305bdc87f63e5eba2b0304d3128683d98f7c409de0b7f9be298d"
	if got := hex.EncodeToString(digest[:]); got != wantDigest {
		t.Fatalf("serialized table changed: got %s, want %s", got, wantDigest)
	}

	compressed := table.EncodeAll(data)
	if got := table.DecodeAll(compressed); !bytes.Equal(got, data) {
		t.Fatal("art_of_war roundtrip mismatch")
	}

	var restored Table
	if err := restored.UnmarshalBinary(serialized); err != nil {
		t.Fatalf("deserialize table: %v", err)
	}
	if got := restored.EncodeAll(data); !bytes.Equal(got, compressed) {
		t.Fatal("serialized table changed compressed output")
	}
}
