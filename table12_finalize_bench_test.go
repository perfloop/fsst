package fsst

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestTable12TrainingIndicesMatchRebuild(t *testing.T) {
	for _, tc := range []struct {
		name    string
		records int
	}{
		{name: "below_sample_target", records: 8},
		{name: "sample_target", records: 32},
		{name: "above_sample_target", records: 64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			inputs := train12FinalizeCorpus(tc.name, tc.records)
			trained := Train12(inputs)

			serialized, err := trained.MarshalBinary()
			if err != nil {
				t.Fatalf("marshal trained table: %v", err)
			}
			var rebuilt Table12
			if err := rebuilt.UnmarshalBinary(serialized); err != nil {
				t.Fatalf("unmarshal trained table: %v", err)
			}

			assertTable12EncoderIndicesEqual(t, trained, &rebuilt)
			if trained.symbols != rebuilt.symbols {
				t.Fatal("trained and rebuilt symbol order differs")
			}
			if trained.nSymbols != rebuilt.nSymbols || trained.lenHisto != rebuilt.lenHisto {
				t.Fatal("trained and rebuilt symbol metadata differs")
			}
			if trained.decLen != rebuilt.decLen || trained.decSymbol != rebuilt.decSymbol {
				t.Fatal("trained and rebuilt decoder tables differ")
			}

			for i, input := range inputs {
				encoded := trained.EncodeAll(input)
				if decoded := trained.DecodeAll(encoded); !bytes.Equal(decoded, input) {
					t.Fatalf("trained table roundtrip mismatch for input %d", i)
				}
				if rebuiltEncoded := rebuilt.EncodeAll(input); !bytes.Equal(rebuiltEncoded, encoded) {
					t.Fatalf("rebuilt table encoding mismatch for input %d", i)
				}
				if decoded := rebuilt.DecodeAll(encoded); !bytes.Equal(decoded, input) {
					t.Fatalf("rebuilt table decode mismatch for input %d", i)
				}
			}
		})
	}
}

func TestTable12AddSymbolIndicesMatchRebuildWithCollision(t *testing.T) {
	first, second := table12HashCollision()
	table := newTable12()
	table.clearSymbols()

	if !table.addSymbol(newSymbolFromByte(0xa5, 0)) {
		t.Fatal("add one-byte symbol")
	}
	if !table.addSymbol(newSymbolFromBytes([]byte{0x5a, 0xc3})) {
		t.Fatal("add two-byte symbol")
	}
	if !table.addSymbol(first) {
		t.Fatal("add first colliding symbol")
	}
	if table.addSymbol(second) {
		t.Fatal("accepted a hash-colliding symbol")
	}

	byteCodes := table.byteCodes
	shortCodes := table.shortCodes
	hashTab := table.hashTab
	table.rebuildIndices()
	if table.byteCodes != byteCodes || table.shortCodes != shortCodes || table.hashTab != hashTab {
		t.Fatal("rebuild changed indices installed by addSymbol")
	}
}

func BenchmarkTrain12Finalization(b *testing.B) {
	for _, tc := range []struct {
		name    string
		records int
	}{
		{name: "below_sample_target", records: 8},
		{name: "diverse_16KiB", records: 32},
		{name: "above_sample_target", records: 64},
	} {
		b.Run(tc.name, func(b *testing.B) {
			inputs := train12FinalizeCorpus(b.Name(), tc.records)
			reference := Train12(inputs)
			want, err := reference.MarshalBinary()
			if err != nil {
				b.Fatalf("marshal reference table: %v", err)
			}
			assertTable12RoundTrip(b, reference, inputs)

			b.ReportAllocs()
			var trained *Table12
			for b.Loop() {
				trained = Train12(inputs)
			}

			got, err := trained.MarshalBinary()
			if err != nil {
				b.Fatalf("marshal trained table: %v", err)
			}
			if !bytes.Equal(got, want) {
				b.Fatal("training produced a different table")
			}
			assertTable12RoundTrip(b, trained, inputs)
		})
	}
}

func BenchmarkTable12RebuildIndices(b *testing.B) {
	inputs := train12FinalizeCorpus(b.Name(), 32)
	table := Train12(inputs)
	assertTable12RoundTrip(b, table, inputs)

	b.ReportAllocs()
	for b.Loop() {
		table.rebuildIndices()
	}
	assertTable12RoundTrip(b, table, inputs)
}

func assertTable12EncoderIndicesEqual(t testing.TB, got, want *Table12) {
	t.Helper()
	if got.byteCodes != want.byteCodes {
		t.Fatal("byte-code indices differ")
	}
	if got.shortCodes != want.shortCodes {
		t.Fatal("short-code indices differ")
	}
	if got.hashTab != want.hashTab {
		t.Fatal("hash indices differ")
	}
}

func assertTable12RoundTrip(t testing.TB, table *Table12, inputs [][]byte) {
	t.Helper()
	for i, input := range inputs {
		encoded := table.EncodeAll(input)
		if decoded := table.DecodeAll(encoded); !bytes.Equal(decoded, input) {
			t.Fatalf("roundtrip mismatch for input %d", i)
		}
	}
}

func table12HashCollision() (symbol, symbol) {
	var (
		seen [fsst12HashSize]symbol
		used [fsst12HashSize]bool
	)
	for value := uint32(0); value <= fsst12HashSize; value++ {
		sym := newSymbolFromBytes([]byte{byte(value), byte(value >> 8), byte(value >> 16)})
		index := sym.hash() & (fsst12HashSize - 1)
		if used[index] {
			return seen[index], sym
		}
		seen[index] = sym
		used[index] = true
	}
	panic("no Table12 hash collision found")
}

func train12FinalizeCorpus(name string, records int) [][]byte {
	const (
		recordSize = 512
		tokenSize  = 8
		repeats    = 8
	)

	seed := uint64(1469598103934665603)
	for i := range len(name) {
		seed ^= uint64(name[i])
		seed *= 1099511628211
	}

	inputs := make([][]byte, records)
	for record := range inputs {
		row := make([]byte, recordSize)
		for group := 0; group < recordSize/(tokenSize*repeats); group++ {
			token := splitMix64(seed + uint64(record*(recordSize/(tokenSize*repeats))+group))
			for repeat := 0; repeat < repeats; repeat++ {
				offset := (group*repeats + repeat) * tokenSize
				binary.LittleEndian.PutUint64(row[offset:offset+tokenSize], token)
			}
		}
		inputs[record] = row
	}
	return inputs
}

func splitMix64(value uint64) uint64 {
	value += 0x9e3779b97f4a7c15
	value = (value ^ (value >> 30)) * 0xbf58476d1ce4e5b9
	value = (value ^ (value >> 27)) * 0x94d049bb133111eb
	return value ^ (value >> 31)
}
