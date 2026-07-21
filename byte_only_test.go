package fsst

import (
	"bytes"
	"testing"
)

const (
	byteOnlyInputSize = 1 << 18
	byteOnlySeed      = uint64(0x9e3779b97f4a7c15)
)

// makeByteOnlyInput produces a high-entropy binary payload that trains a table
// containing only one-byte symbols. The seed is an argument so the benchmark
// data is constructed from runtime state rather than a compile-time constant.
func makeByteOnlyInput(size int, seed uint64) []byte {
	data := make([]byte, size)
	state := seed
	for i := range data {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		data[i] = byte(state >> 24)
	}
	return data
}

func requireByteOnlyTable(tb testing.TB, table *Table) {
	tb.Helper()
	if table.lenHisto[0] != table.nSymbols || table.suffixLim != 0 {
		tb.Fatalf("not byte-only: nSymbols=%d lenHisto=%v suffixLim=%d", table.nSymbols, table.lenHisto, table.suffixLim)
	}
	for length, count := range table.lenHisto[1:] {
		if count != 0 {
			tb.Fatalf("byte-only table has %d symbols of length %d", count, length+2)
		}
	}
}

func requireMultiByteTable(tb testing.TB, table *Table) {
	tb.Helper()
	for _, count := range table.lenHisto[1:] {
		if count != 0 {
			return
		}
	}
	tb.Fatalf("control table has no multibyte symbols: nSymbols=%d lenHisto=%v", table.nSymbols, table.lenHisto)
}

// encodeByteOnlyReference is the semantic encoder for tables without any
// multibyte symbols. It intentionally touches only byteCodes, so it can serve
// as an independent oracle for the generic encoder and a future specialized
// path.
func encodeByteOnlyReference(table *Table, input []byte) []byte {
	output := make([]byte, 0, 2*len(input))
	for _, value := range input {
		code := table.byteCodes[value]
		output = append(output, byte(code))
		if code&codeBase != 0 {
			output = append(output, value)
		}
	}
	return output
}

func assertByteOnlyEncoding(tb testing.TB, table *Table, input []byte) {
	tb.Helper()
	for _, size := range []int{0, 1, 7, 8, 9, 510, 511, 512, 513, len(input)} {
		input := input[:size]
		want := encodeByteOnlyReference(table, input)

		if got := table.EncodeAll(input); !bytes.Equal(got, want) {
			tb.Fatalf("EncodeAll(%d) = %x, want byte-only output %x", size, got, want)
		}

		buffer := make([]byte, 0, 2*len(input)+outputPadding)
		got := table.EncodeInto(buffer, input)
		if !bytes.Equal(got, want) {
			tb.Fatalf("EncodeInto(%d) = %x, want byte-only output %x", size, got, want)
		}
		if decoded := table.DecodeAll(got); !bytes.Equal(decoded, input) {
			tb.Fatalf("DecodeAll(EncodeInto(%d)) = %x, want %x", size, decoded, input)
		}
	}
}

// TestByteOnlyTableEncodeEquivalence is a controlled experiment for the
// byte-only table condition: the generic encoder must produce exactly the
// byteCodes-only output across chunk boundaries, both before and after table
// serialization. It also covers the zero-symbol table, which satisfies the
// same predicate and uses escape codes for every byte.
func TestByteOnlyTableEncodeEquivalence(t *testing.T) {
	input := makeByteOnlyInput(byteOnlyInputSize, byteOnlySeed)
	trained := Train([][]byte{input})

	serialized, err := trained.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal trained table: %v", err)
	}
	var restored Table
	if err := restored.UnmarshalBinary(serialized); err != nil {
		t.Fatalf("restore trained table: %v", err)
	}

	trainedOutput := encodeByteOnlyReference(trained, input)
	if restoredOutput := encodeByteOnlyReference(&restored, input); !bytes.Equal(restoredOutput, trainedOutput) {
		t.Fatal("restored byte-only table changed compressed output")
	}

	for _, test := range []struct {
		name  string
		table *Table
	}{
		{name: "trained", table: trained},
		{name: "restored", table: &restored},
		{name: "empty_training", table: Train(nil)},
	} {
		t.Run(test.name, func(t *testing.T) {
			requireByteOnlyTable(t, test.table)
			t.Logf("byte-only table: nSymbols=%d lenHisto=%v suffixLim=%d", test.table.nSymbols, test.table.lenHisto, test.table.suffixLim)
			assertByteOnlyEncoding(t, test.table, input)
		})
	}
}

func benchmarkEncodeInto(b *testing.B, table *Table, input, want []byte) {
	b.Helper()
	buffer := make([]byte, 0, 2*len(input)+outputPadding)
	if got := table.EncodeInto(buffer, input); !bytes.Equal(got, want) {
		b.Fatal("pre-benchmark EncodeInto output mismatch")
	}

	var output []byte
	b.ReportAllocs()
	b.SetBytes(int64(len(input)))
	b.ResetTimer()
	for b.Loop() {
		output = table.EncodeInto(buffer, input)
	}
	b.StopTimer()

	if !bytes.Equal(output, want) {
		b.Fatal("post-benchmark EncodeInto output mismatch")
	}
}

func BenchmarkTableEncodeByteOnly(b *testing.B) {
	b.Run("ByteOnly", func(b *testing.B) {
		input := makeByteOnlyInput(byteOnlyInputSize, byteOnlySeed^uint64(len(b.Name())))
		table := Train([][]byte{input})
		requireByteOnlyTable(b, table)
		benchmarkEncodeInto(b, table, input, encodeByteOnlyReference(table, input))
	})

	b.Run("MultiByteControl", func(b *testing.B) {
		input := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog "), 16)
		table := Train([][]byte{input})
		requireMultiByteTable(b, table)
		benchmarkEncodeInto(b, table, input, table.EncodeAll(input))
	})
}
