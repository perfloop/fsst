package fsst

import (
	"bytes"
	"testing"
)

const table12DecodeProbeBytes = 9

func table12LongTableLiteralFixture(tb testing.TB) (*Table12, []byte, []byte) {
	tb.Helper()

	training := table12ExpandedInput()
	table := Train12([][]byte{training})
	trainingEncoded := table.EncodeAll(training)
	if len(training) <= len(trainingEncoded)*2+8 {
		tb.Fatalf("training fixture does not demonstrate long-symbol expansion: input=%d encoded=%d", len(training), len(trainingEncoded))
	}
	if got := table.DecodeAll(trainingEncoded[:table12DecodeProbeBytes]); len(got) <= table12DecodeProbeBytes*2 {
		tb.Fatalf("training prefix does not demonstrate expansion: decoded=%d encoded=%d", len(got), table12DecodeProbeBytes)
	}

	input := table12LiteralInput(48 * 1024)
	encoded := table.EncodeAll(input)
	if len(input) > len(encoded)*2+8 {
		tb.Fatalf("literal payload unexpectedly exceeds the 2x estimate: input=%d encoded=%d", len(input), len(encoded))
	}
	if got := table.DecodeAll(encoded[:table12DecodeProbeBytes]); len(got) > table12DecodeProbeBytes*2 {
		tb.Fatalf("literal prefix unexpectedly demonstrates expansion: decoded=%d encoded=%d", len(got), table12DecodeProbeBytes)
	}
	if got := table.DecodeAll(encoded); !bytes.Equal(got, input) {
		tb.Fatal("long-table literal DecodeAll roundtrip mismatch")
	}
	return table, input, encoded
}

func TestTable12DecodeLongTableNonExpandingAndTail(t *testing.T) {
	table, input, encoded := table12LongTableLiteralFixture(t)
	if got := table.DecodeAll(encoded); !bytes.Equal(got, input) {
		t.Fatal("long-table literal DecodeAll roundtrip mismatch")
	}

	oddTail := append(append([]byte(nil), table12ExpandedInput()...), '!')
	oddEncoded := table.EncodeAll(oddTail)
	if len(oddEncoded)%3 != 2 {
		t.Fatalf("odd-code input encoded to %d bytes, want a two-byte packed tail", len(oddEncoded))
	}
	if len(oddTail) <= len(oddEncoded)*2+8 {
		t.Fatalf("odd-code input does not exceed the 2x estimate: input=%d encoded=%d", len(oddTail), len(oddEncoded))
	}
	if got := table.DecodeAll(oddEncoded); !bytes.Equal(got, oddTail) {
		t.Fatal("odd-code long-symbol DecodeAll roundtrip mismatch")
	}

	deferredExpansion := append(table12LiteralInput(6), table12ExpandedInput()...)
	deferredEncoded := table.EncodeAll(deferredExpansion)
	if len(deferredExpansion) <= len(deferredEncoded)*2+8 {
		t.Fatalf("deferred-expansion input does not exceed the 2x estimate: input=%d encoded=%d", len(deferredExpansion), len(deferredEncoded))
	}
	if got := table.DecodeAll(deferredEncoded[:table12DecodeProbeBytes]); len(got) > table12DecodeProbeBytes*2 {
		t.Fatalf("deferred-expansion prefix unexpectedly demonstrates expansion: decoded=%d encoded=%d", len(got), table12DecodeProbeBytes)
	}
	if got := table.DecodeAll(deferredEncoded); !bytes.Equal(got, deferredExpansion) {
		t.Fatal("deferred-expansion DecodeAll roundtrip mismatch")
	}
}

func BenchmarkTable12DecodeLongTableLiteral(b *testing.B) {
	table, input, encoded := table12LongTableLiteralFixture(b)

	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	var decoded []byte
	for b.Loop() {
		decoded = table.DecodeAll(encoded)
	}
	b.StopTimer()
	if !bytes.Equal(decoded, input) {
		b.Fatal("long-table literal DecodeAll roundtrip mismatch after timing")
	}
}
