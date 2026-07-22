package fsst

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
)

const artOfWarSelectionTableSHA256 = "682a257952d7305bdc87f63e5eba2b0304d3128683d98f7c409de0b7f9be298d"

// TestTrainSelectionPreservesArtOfWarTable locks the public Train result for a
// corpus whose repeated training rounds exercise candidate selection above its
// retention limit. The table digest checks the serialized bytes exactly for
// practical purposes, while the round trips cover both the trained and restored
// table representations.
func TestTrainSelectionPreservesArtOfWarTable(t *testing.T) {
	data, err := os.ReadFile("testdata/art_of_war.txt")
	if err != nil {
		t.Fatal(err)
	}

	table := Train([][]byte{data})
	var serialized bytes.Buffer
	if _, err := table.WriteTo(&serialized); err != nil {
		t.Fatalf("serialize trained table: %v", err)
	}
	gotDigest := sha256.Sum256(serialized.Bytes())
	if got := hex.EncodeToString(gotDigest[:]); got != artOfWarSelectionTableSHA256 {
		t.Fatalf("serialized table digest changed: got %s, want %s", got, artOfWarSelectionTableSHA256)
	}

	encoded := table.EncodeAll(data)
	if decoded := table.DecodeAll(encoded); !bytes.Equal(decoded, data) {
		t.Fatal("trained table did not round-trip the corpus")
	}

	var restored Table
	if _, err := restored.ReadFrom(bytes.NewReader(serialized.Bytes())); err != nil {
		t.Fatalf("restore serialized table: %v", err)
	}
	if reencoded := restored.EncodeAll(data); !bytes.Equal(reencoded, encoded) {
		t.Fatal("restored table changed the compressed corpus")
	}
	if decoded := restored.DecodeAll(encoded); !bytes.Equal(decoded, data) {
		t.Fatal("restored table did not round-trip the corpus")
	}
}
