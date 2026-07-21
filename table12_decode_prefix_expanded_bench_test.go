package fsst

import (
	"bytes"
	"testing"
)

const table12PrefixProbeBytes = 9

// table12PrefixExpandedLiteralTailFixture produces a valid packed stream whose
// first six codes expand past the normal 2x estimate while its complete output
// still fits that estimate. The tail is encoded separately on a three-byte
// boundary, so it begins after the expanding prefix without changing its codes.
func table12PrefixExpandedLiteralTailFixture(tb testing.TB, tailSize int) (*Table12, []byte, []byte) {
	tb.Helper()

	training := table12ExpandedInput()
	table := Train12([][]byte{training})
	trainingEncoded := table.EncodeAll(training)
	prefix := append([]byte(nil), trainingEncoded[:table12PrefixProbeBytes]...)
	prefixDecoded := table.DecodeAll(prefix)
	if len(prefixDecoded) <= len(prefix)*2 {
		tb.Fatalf("prefix does not exceed the 2x estimate: decoded=%d encoded=%d", len(prefixDecoded), len(prefix))
	}

	tail := table12LiteralInput(tailSize)
	tailEncoded := table.EncodeAll(tail)
	if len(tailEncoded)%3 != 0 {
		tb.Fatalf("literal tail has incomplete packed group: encoded=%d", len(tailEncoded))
	}

	input := make([]byte, len(prefixDecoded)+len(tail))
	copy(input, prefixDecoded)
	copy(input[len(prefixDecoded):], tail)
	encoded := make([]byte, len(prefix)+len(tailEncoded))
	copy(encoded, prefix)
	copy(encoded[len(prefix):], tailEncoded)
	if len(input) > len(encoded)*2+8 {
		tb.Fatalf("mixed payload unexpectedly exceeds the 2x estimate: input=%d encoded=%d", len(input), len(encoded))
	}
	if got := table.DecodeAll(encoded); !bytes.Equal(got, input) {
		tb.Fatal("mixed DecodeAll roundtrip mismatch")
	}
	return table, input, encoded
}

func TestTable12DecodePrefixExpandedLiteralTail(t *testing.T) {
	for _, tc := range []struct {
		name string
		size int
	}{
		{name: "4KiB", size: 4 * 1024},
		{name: "16KiB", size: 16 * 1024},
		{name: "48KiB", size: 48 * 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table, input, encoded := table12PrefixExpandedLiteralTailFixture(t, tc.size)

			exact := make([]byte, len(encoded))
			copy(exact, encoded)
			retained := make([]byte, len(encoded), len(encoded)*4)
			copy(retained, encoded)

			decodedExact := table.DecodeAll(exact)
			decodedRetained := table.DecodeAll(retained)
			if !bytes.Equal(decodedExact, input) || !bytes.Equal(decodedRetained, input) {
				t.Fatal("DecodeAll changed output for an equivalent retained source")
			}
			if cap(decodedExact) != cap(decodedRetained) {
				t.Fatalf("DecodeAll output capacity depends on source capacity: exact=%d retained=%d", cap(decodedExact), cap(decodedRetained))
			}
		})
	}
}

func BenchmarkTable12DecodePrefixExpandedLiteralTail(b *testing.B) {
	table, input, encoded := table12PrefixExpandedLiteralTailFixture(b, 48*1024)

	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	var decoded []byte
	for b.Loop() {
		decoded = table.DecodeAll(encoded)
	}
	b.StopTimer()
	if !bytes.Equal(decoded, input) {
		b.Fatal("mixed DecodeAll roundtrip mismatch after timing")
	}
}
