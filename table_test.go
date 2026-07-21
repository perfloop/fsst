package fsst

import (
	"bytes"
	"encoding/binary"
	"strconv"
	"strings"
	"testing"
)

func TestTableAddFind(t *testing.T) {
	tbl := newTable()
	s1 := newSymbolFromBytes([]byte{'x'})
	if !tbl.addSymbol(s1) {
		t.Fatalf("add single-byte")
	}
	s2 := newSymbolFromBytes([]byte{'a', 'b'})
	if !tbl.addSymbol(s2) {
		t.Fatalf("add two-byte")
	}
	s3 := newSymbolFromBytes([]byte{'a', 'b', 'c'})
	if !tbl.addSymbol(s3) {
		t.Fatalf("add long")
	}

	// find longest for prefix "abc..."
	code := tbl.findLongestSymbol(newSymbolFromBytes([]byte{'a', 'b', 'c', 'd'}))
	got := tbl.symbols[code]
	if got.length() < 2 {
		t.Fatalf("expected len>=2 got %d", got.length())
	}
}

func TestFinalize(t *testing.T) {
	tbl := newTable()
	tbl.addSymbol(newSymbolFromBytes([]byte{'a'}))
	tbl.addSymbol(newSymbolFromBytes([]byte{'b', 'c'}))
	tbl.addSymbol(newSymbolFromBytes([]byte{'d', 'e', 'f'}))
	tbl.finalize()
	if tbl.nSymbols == 0 {
		t.Fatalf("no symbols after finalize")
	}
	// shortCodes for unknown 2-byte pattern must map to byteCodes of first byte
	sc := tbl.shortCodes[int('Z')<<8|int('Q')]
	if (sc&codeMask) >= codeBase && sc>>lenBits != 1 {
		t.Fatalf("shortCodes not patched for single byte fallback")
	}
}

func TestRebuildTableRoundtrip(t *testing.T) {
	input := []byte("When in the Course of human events, it becomes necessary for one people to dissolve")
	tbl := Train([][]byte{input})
	var buf bytes.Buffer
	if _, err := tbl.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	var tbl2 Table
	if _, err := tbl2.ReadFrom(&buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	comp := tbl2.EncodeAll(input)
	got := tbl2.DecodeAll(comp)
	if !bytes.Equal(got, input) {
		t.Fatalf("rebuild roundtrip mismatch")
	}
}

func byteOnlyFixture(size int) []byte {
	data := make([]byte, size)
	state := uint64(0x9e3779b97f4a7c15)
	for i := range data {
		state ^= state << 13
		state ^= state >> 7
		state ^= state << 17
		data[i] = byte(state >> 24)
	}
	return data
}

func byteOnlyOutput(table *Table, input []byte) []byte {
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

func TestByteOnlyTableEncoding(t *testing.T) {
	input := byteOnlyFixture(1 << 18)
	trained := Train([][]byte{input})
	serialized, err := trained.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var restored Table
	if err := restored.UnmarshalBinary(serialized); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, test := range []struct {
		name  string
		table *Table
	}{
		{name: "trained", table: trained},
		{name: "restored", table: &restored},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.table.lenHisto[0] != test.table.nSymbols || test.table.suffixLim != 0 {
				t.Fatalf("not byte-only: nSymbols=%d lenHisto=%v suffixLim=%d", test.table.nSymbols, test.table.lenHisto, test.table.suffixLim)
			}
			for _, size := range []int{0, 1, 7, 8, 9, 510, 511, 512, 513, len(input)} {
				input := input[:size]
				want := byteOnlyOutput(test.table, input)
				got := test.table.EncodeInto(make([]byte, 0, 2*len(input)+outputPadding), input)
				if !bytes.Equal(got, want) {
					t.Fatalf("EncodeInto(%d) = %x, want %x", size, got, want)
				}
				if decoded := test.table.DecodeAll(got); !bytes.Equal(decoded, input) {
					t.Fatalf("DecodeAll(EncodeInto(%d)) = %x, want %x", size, decoded, input)
				}
			}
		})
	}
}

func genericChunkEncoding(table *Table, input []byte) []byte {
	padded := make([]byte, len(input)+chunkPadding)
	copy(padded, input)
	output := make([]byte, 2*len(input)+outputPadding)
	return output[:table.encodeChunk(output, 0, padded, len(input))]
}

func TestByteOnlyImportedTableEncoding(t *testing.T) {
	data := make([]byte, 8+8+2)
	binary.LittleEndian.PutUint64(data[:8], (tableVersion<<32)|(uint64(2)<<8)|1)
	data[8] = 2
	data[16] = 'a'
	data[17] = 'a'

	var table Table
	if _, err := table.ReadFrom(bytes.NewReader(data)); err != nil {
		t.Fatalf("ReadFrom imported table: %v", err)
	}
	if table.lenHisto[0] != table.nSymbols || table.suffixLim != 0 {
		t.Fatalf("not byte-only: nSymbols=%d lenHisto=%v suffixLim=%d", table.nSymbols, table.lenHisto, table.suffixLim)
	}

	input := []byte{'a', 'x', 'a', 'y', 'a', 'x', 'a', 'z'}
	want := genericChunkEncoding(&table, input)
	if got := table.EncodeAll(input); !bytes.Equal(got, want) {
		t.Fatalf("EncodeAll = %x, want generic output %x", got, want)
	}
	got := table.EncodeInto(make([]byte, 0, 2*len(input)+outputPadding), input)
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeInto = %x, want generic output %x", got, want)
	}
	if decoded := table.DecodeAll(got); !bytes.Equal(decoded, input) {
		t.Fatalf("DecodeAll(EncodeInto) = %x, want %x", decoded, input)
	}
}

func TestByteOnlyTableOverlappingBuffer(t *testing.T) {
	seed := []byte{0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x0f}
	for _, method := range []struct {
		name   string
		encode func(*Table, []byte, []byte) []byte
	}{
		{name: "Encode", encode: func(table *Table, buf, input []byte) []byte { return table.Encode(buf, input) }},
		{name: "EncodeInto", encode: func(table *Table, buf, input []byte) []byte { return table.EncodeInto(buf, input) }},
	} {
		for _, outputOffset := range []int{0, 1} {
			for size := 2; size <= len(seed); size++ {
				t.Run(method.name+"/offset_"+strconv.Itoa(outputOffset)+"/"+strconv.Itoa(size), func(t *testing.T) {
					table := Train(nil)
					storage := make([]byte, 2*size+outputPadding+outputOffset)
					copy(storage, seed[:size])
					original := bytes.Clone(storage[:size])
					got := method.encode(table, storage[outputOffset:outputOffset], storage[:size])
					if decoded := table.DecodeAll(got); !bytes.Equal(decoded, original) {
						t.Fatalf("DecodeAll(overlapping output) = %x, want %x", decoded, original)
					}
				})
			}
		}
	}
}

func BenchmarkByteOnlyEncoding(b *testing.B) {
	input := byteOnlyFixture(1 << 18)
	table := Train([][]byte{input})
	if table.lenHisto[0] != table.nSymbols || table.suffixLim != 0 {
		b.Fatalf("not byte-only: nSymbols=%d lenHisto=%v suffixLim=%d", table.nSymbols, table.lenHisto, table.suffixLim)
	}
	want := byteOnlyOutput(table, input)
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

// TestTableLimits tests table behavior at limits
func TestTableLimits(t *testing.T) {
	// Test with many unique patterns to approach symbol limit
	var inputs [][]byte
	for i := 0; i < 300; i++ {
		inputs = append(inputs, []byte(strings.Repeat(string(rune('a'+i%26)), i%8+1)))
	}

	tbl := Train(inputs)
	// Verify it still works
	comp := tbl.EncodeAll(inputs[0])
	got := tbl.DecodeAll(comp)
	if !bytes.Equal(got, inputs[0]) {
		t.Fatalf("roundtrip failed with many symbols")
	}
}

// TestDecodeAPIs tests all decode variants
func TestDecodeAPIs(t *testing.T) {
	input := []byte("Hello, World! This is a test message for FSST compression.")
	tbl := Train([][]byte{input})
	comp := tbl.EncodeAll(input)

	// Test DecodeAll
	t.Run("DecodeAll", func(t *testing.T) {
		got := tbl.DecodeAll(comp)
		if !bytes.Equal(got, input) {
			t.Fatalf("DecodeAll mismatch: got %q, want %q", got, input)
		}
	})

	// Test Decode with sufficient buffer
	t.Run("Decode_sufficient", func(t *testing.T) {
		buf := make([]byte, len(input)*2) // Generous buffer
		got := tbl.Decode(buf, comp)
		if !bytes.Equal(got, input) {
			t.Fatalf("Decode mismatch: got %q, want %q", got, input)
		}
	})

	// Test Decode with small buffer (should grow)
	t.Run("Decode_small", func(t *testing.T) {
		buf := make([]byte, 5) // Too small
		got := tbl.Decode(buf, comp)
		if !bytes.Equal(got, input) {
			t.Fatalf("Decode mismatch: got %q, want %q", got, input)
		}
	})

	// Test Decode with nil buffer (should allocate)
	t.Run("Decode_nil", func(t *testing.T) {
		got := tbl.Decode(nil, comp)
		if !bytes.Equal(got, input) {
			t.Fatalf("Decode mismatch: got %q, want %q", got, input)
		}
	})

	// Test DecodeString
	t.Run("DecodeString", func(t *testing.T) {
		compStr := string(comp)
		got := tbl.DecodeString(compStr)
		if !bytes.Equal(got, input) {
			t.Fatalf("DecodeString mismatch: got %q, want %q", got, input)
		}
	})

}

// TestReadFromMalformed verifies that ReadFrom rejects crafted inputs
// without panicking.
func TestReadFromMalformed(t *testing.T) {
	// Helper: build a valid serialized table to use as a base.
	validTable := func() []byte {
		tbl := Train([][]byte{[]byte("hello world hello")})
		var buf bytes.Buffer
		tbl.WriteTo(&buf)
		return buf.Bytes()
	}

	t.Run("bad_version", func(t *testing.T) {
		data := validTable()
		// Corrupt version (first 4 bytes of the 8-byte header)
		data[4] = 0xFF
		var tbl Table
		_, err := tbl.ReadFrom(bytes.NewReader(data))
		if err != ErrBadVersion {
			t.Fatalf("expected ErrBadVersion, got %v", err)
		}
	})

	t.Run("lenHisto_sum_exceeds_nSymbols", func(t *testing.T) {
		data := validTable()
		// lenHisto starts at byte 8, 8 bytes long.
		// Set all histo entries to 255 so sum far exceeds nSymbols.
		for i := 8; i < 16; i++ {
			data[i] = 255
		}
		var tbl Table
		_, err := tbl.ReadFrom(bytes.NewReader(data))
		if err != ErrCorrupted {
			t.Fatalf("expected ErrCorrupted, got %v", err)
		}
	})

	t.Run("lenHisto_sum_less_than_nSymbols", func(t *testing.T) {
		data := validTable()
		// Zero out lenHisto so sum=0 but nSymbols>0.
		for i := 8; i < 16; i++ {
			data[i] = 0
		}
		var tbl Table
		_, err := tbl.ReadFrom(bytes.NewReader(data))
		if err != ErrCorrupted {
			t.Fatalf("expected ErrCorrupted, got %v", err)
		}
	})

	t.Run("truncated_input", func(t *testing.T) {
		data := validTable()
		// Truncate to just the header
		var tbl Table
		_, err := tbl.ReadFrom(bytes.NewReader(data[:8]))
		if err == nil {
			t.Fatalf("expected error on truncated input")
		}
	})
}

// TestDecodeBatch tests the batch decode API
func TestDecodeBatch(t *testing.T) {
	strings := [][]byte{
		[]byte("Hello, World!"),
		[]byte("FSST compression is fast"),
		[]byte(`{"name":"Alice","age":30}`),
		[]byte(""),
		[]byte("x"),
		[]byte("The quick brown fox jumps over the lazy dog"),
	}

	tbl := Train(strings)

	// Encode all strings and build concatenated src + offsets
	var src []byte
	offsets := []int{0}
	for _, s := range strings {
		comp := tbl.EncodeAll(s)
		src = append(src, comp...)
		offsets = append(offsets, len(src))
	}

	t.Run("basic_roundtrip", func(t *testing.T) {
		dst, dstOffsets, err := tbl.DecodeBatch(nil, nil, src, offsets)
		if err != nil {
			t.Fatal(err)
		}
		if len(dstOffsets) != len(offsets) {
			t.Fatalf("expected %d offsets, got %d", len(offsets), len(dstOffsets))
		}
		for i, s := range strings {
			got := dst[dstOffsets[i]:dstOffsets[i+1]]
			if !bytes.Equal(got, s) {
				t.Fatalf("string %d: got %q, want %q", i, got, s)
			}
		}
	})

	t.Run("with_buffer", func(t *testing.T) {
		buf := make([]byte, 1024)
		offsetsBuf := make([]int, len(offsets))
		dst, dstOffsets, err := tbl.DecodeBatch(buf, offsetsBuf, src, offsets)
		if err != nil {
			t.Fatal(err)
		}
		for i, s := range strings {
			got := dst[dstOffsets[i]:dstOffsets[i+1]]
			if !bytes.Equal(got, s) {
				t.Fatalf("string %d: got %q, want %q", i, got, s)
			}
		}
	})

	t.Run("small_buffer_grows", func(t *testing.T) {
		buf := make([]byte, 4)
		dst, dstOffsets, err := tbl.DecodeBatch(buf, nil, src, offsets)
		if err != nil {
			t.Fatal(err)
		}
		for i, s := range strings {
			got := dst[dstOffsets[i]:dstOffsets[i+1]]
			if !bytes.Equal(got, s) {
				t.Fatalf("string %d: got %q, want %q", i, got, s)
			}
		}
	})

	t.Run("empty_batch", func(t *testing.T) {
		dst, dstOffsets, err := tbl.DecodeBatch(nil, nil, nil, []int{0})
		if err != nil {
			t.Fatal(err)
		}
		if len(dst) != 0 {
			t.Fatalf("expected empty dst, got %d bytes", len(dst))
		}
		if len(dstOffsets) != 1 {
			t.Fatalf("expected 1 offset, got %d", len(dstOffsets))
		}
	})

	t.Run("single_string", func(t *testing.T) {
		comp := tbl.EncodeAll(strings[0])
		dst, dstOffsets, err := tbl.DecodeBatch(nil, nil, comp, []int{0, len(comp)})
		if err != nil {
			t.Fatal(err)
		}
		got := dst[dstOffsets[0]:dstOffsets[1]]
		if !bytes.Equal(got, strings[0]) {
			t.Fatalf("got %q, want %q", got, strings[0])
		}
	})

	t.Run("invalid_offsets", func(t *testing.T) {
		for _, invalid := range [][]int{nil, {1}, {0, len(src) + 1}, {0, 2, 1}, {0, len(src) - 1}} {
			if _, _, err := tbl.DecodeBatch(nil, nil, src, invalid); err != ErrCorrupted {
				t.Fatalf("offsets %v: got %v, want ErrCorrupted", invalid, err)
			}
		}
	})

	t.Run("escape_does_not_cross_string_boundary", func(t *testing.T) {
		if _, _, err := tbl.DecodeBatch(nil, nil, []byte{escapeCode, 'x'}, []int{0, 1, 2}); err != ErrCorrupted {
			t.Fatalf("got %v, want ErrCorrupted", err)
		}
	})

	t.Run("reuses_buffers", func(t *testing.T) {
		dst := make([]byte, 0, 1024)
		dstOffsets := make([]int, 0, len(offsets))
		allocs := testing.AllocsPerRun(100, func() {
			var err error
			dst, dstOffsets, err = tbl.DecodeBatch(dst, dstOffsets, src, offsets)
			if err != nil {
				panic(err)
			}
		})
		if allocs != 0 {
			t.Fatalf("DecodeBatch allocated %v times, want 0", allocs)
		}
	})

	t.Run("reuses_source_offsets", func(t *testing.T) {
		reusedOffsets := append([]int(nil), offsets...)
		dst, dstOffsets, err := tbl.DecodeBatch(nil, reusedOffsets, src, reusedOffsets)
		if err != nil {
			t.Fatal(err)
		}
		for i, s := range strings {
			got := dst[dstOffsets[i]:dstOffsets[i+1]]
			if !bytes.Equal(got, s) {
				t.Fatalf("string %d: got %q, want %q", i, got, s)
			}
		}
	})
}

// BenchmarkDecode benchmarks different decode scenarios
func BenchmarkDecode(b *testing.B) {
	inputs := []struct {
		name string
		data []byte
	}{
		{"small_100B", bytes.Repeat([]byte("hello world "), 8)},
		{"medium_1KB", bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. "), 22)},
		{"large_10KB", bytes.Repeat([]byte("FSST compression algorithm for structured text data. "), 192)},
		{"json_like", bytes.Repeat([]byte(`{"name":"John","age":30,"city":"New York","active":true}`), 10)},
		{"repetitive", bytes.Repeat([]byte("aaaaaaaaaa"), 100)},
	}

	for _, input := range inputs {
		tbl := Train([][]byte{input.data})
		comp := tbl.EncodeAll(input.data)

		b.Run(input.name+"/DecodeAll", func(b *testing.B) {
			b.SetBytes(int64(len(input.data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tbl.DecodeAll(comp)
			}
		})

		b.Run(input.name+"/Decode_with_buf", func(b *testing.B) {
			buf := make([]byte, len(input.data)*2)
			b.SetBytes(int64(len(input.data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tbl.Decode(buf, comp)
			}
		})

		b.Run(input.name+"/Decode_nil", func(b *testing.B) {
			b.SetBytes(int64(len(input.data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tbl.Decode(nil, comp)
			}
		})

		b.Run(input.name+"/DecodeString", func(b *testing.B) {
			compStr := string(comp)
			b.SetBytes(int64(len(input.data)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = tbl.DecodeString(compStr)
			}
		})
	}
}

// BenchmarkDecodeBatch compares per-string DecodeAll vs batch decode.
func BenchmarkDecodeBatch(b *testing.B) {
	// Generate 1000 strings of varying sizes
	baseStrings := []string{
		`{"name":"Alice","age":30,"city":"New York"}`,
		`{"name":"Bob","age":25,"city":"San Francisco"}`,
		`{"id":123,"type":"event","ts":"2024-01-15T10:30:00Z"}`,
		`SELECT * FROM users WHERE active = true`,
		`https://example.com/api/v2/users?page=1&limit=100`,
	}

	const numStrings = 1000
	inputs := make([][]byte, numStrings)
	for i := range numStrings {
		inputs[i] = []byte(baseStrings[i%len(baseStrings)])
	}

	tbl := Train(inputs)

	// Encode all and build batch structures
	compressed := make([][]byte, numStrings)
	var batchSrc []byte
	batchOffsets := make([]int, numStrings+1)
	var totalDecompressed int64
	for i, input := range inputs {
		compressed[i] = tbl.EncodeAll(input)
		batchOffsets[i] = len(batchSrc)
		batchSrc = append(batchSrc, compressed[i]...)
		totalDecompressed += int64(len(input))
	}
	batchOffsets[numStrings] = len(batchSrc)

	b.Run("per_string_DecodeAll", func(b *testing.B) {
		b.SetBytes(totalDecompressed)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, c := range compressed {
				_ = tbl.DecodeAll(c)
			}
		}
	})

	b.Run("per_string_Decode_reuse", func(b *testing.B) {
		buf := make([]byte, 4096)
		b.SetBytes(totalDecompressed)
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for _, c := range compressed {
				_ = tbl.Decode(buf, c)
			}
		}
	})

	b.Run("DecodeBatch", func(b *testing.B) {
		dst := make([]byte, 0, totalDecompressed)
		dstOffsets := make([]int, 0, len(batchOffsets))
		var err error
		b.SetBytes(totalDecompressed)
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			dst, dstOffsets, err = tbl.DecodeBatch(dst, dstOffsets, batchSrc, batchOffsets)
		}
		b.StopTimer()
		if err != nil {
			b.Fatal(err)
		}
	})
}
