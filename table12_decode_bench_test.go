package fsst

import (
	"bytes"
	"testing"
)

const table12DecodePhrase = "the quick brown fox jumps over the lazy dog "

func table12ExpandedInput() []byte {
	return bytes.Repeat([]byte(table12DecodePhrase), 1000)
}

// table12LiteralInput produces a deterministic, literal-heavy input. Its
// high-entropy byte stream avoids the repeated multi-byte patterns that make
// the expanded fixture compress substantially.
func table12LiteralInput(n int) []byte {
	input := make([]byte, n)
	state := uint64(0x9e3779b97f4a7c15)
	for i := range input {
		state = state*6364136223846793005 + 1442695040888963407
		input[i] = byte(state >> 56)
	}
	return input
}

func TestTable12DecodeBufferPaths(t *testing.T) {
	tests := []struct {
		name         string
		input        []byte
		shouldExpand bool
	}{
		{
			name:         "expanded",
			input:        table12ExpandedInput(),
			shouldExpand: true,
		},
		{
			name:         "literal_heavy",
			input:        table12LiteralInput(48 * 1024),
			shouldExpand: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			table := Train12([][]byte{tc.input})
			encoded := table.EncodeAll(tc.input)
			initialCapacity := len(encoded)*2 + 8
			if got := len(tc.input) > initialCapacity; got != tc.shouldExpand {
				t.Fatalf("fixture resize condition = %t (input=%d encoded=%d initial-capacity=%d), want %t", got, len(tc.input), len(encoded), initialCapacity, tc.shouldExpand)
			}

			if got := table.DecodeAll(encoded); !bytes.Equal(got, tc.input) {
				t.Fatal("DecodeAll roundtrip mismatch")
			}

			provided := make([]byte, 4, len(tc.input)+7)
			got := table.Decode(provided, encoded)
			if !bytes.Equal(got, tc.input) {
				t.Fatal("Decode with sufficiently large buffer mismatch")
			}
			if &got[0] != &provided[0] {
				t.Fatal("Decode did not reuse a sufficiently large supplied buffer")
			}

			small := make([]byte, 0, 8)
			got = table.Decode(small, encoded)
			if !bytes.Equal(got, tc.input) {
				t.Fatal("Decode with small buffer mismatch")
			}
			if &got[0] == &small[:cap(small)][0] {
				t.Fatal("Decode did not grow an insufficient supplied buffer")
			}
		})
	}
}

func benchmarkTable12Decode(b *testing.B, input []byte, shouldExpand bool) {
	b.Helper()

	table := Train12([][]byte{input})
	encoded := table.EncodeAll(input)
	initialCapacity := len(encoded)*2 + 8
	if got := len(input) > initialCapacity; got != shouldExpand {
		b.Fatalf("fixture resize condition = %t (input=%d encoded=%d initial-capacity=%d), want %t", got, len(input), len(encoded), initialCapacity, shouldExpand)
	}
	if got := table.DecodeAll(encoded); !bytes.Equal(got, input) {
		b.Fatal("DecodeAll roundtrip mismatch before timing")
	}

	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	var decoded []byte
	for b.Loop() {
		decoded = table.DecodeAll(encoded)
	}
	b.StopTimer()
	if !bytes.Equal(decoded, input) {
		b.Fatal("DecodeAll roundtrip mismatch after timing")
	}
}

func BenchmarkTable12DecodeExpanded(b *testing.B) {
	benchmarkTable12Decode(b, table12ExpandedInput(), true)
}

func BenchmarkTable12DecodeLiteral(b *testing.B) {
	benchmarkTable12Decode(b, table12LiteralInput(48*1024), false)
}
