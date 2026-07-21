package fsst

import (
	"bytes"
	"testing"
)

func TestTable12Basic(t *testing.T) {
	data := [][]byte{
		[]byte("hello world hello world hello"),
		[]byte("the quick brown fox jumps over the lazy dog"),
		[]byte("lorem ipsum dolor sit amet consectetur"),
	}

	tbl := Train12(data)
	if tbl.nSymbols == 0 {
		t.Fatal("expected some learned symbols")
	}

	for _, input := range data {
		encoded := tbl.EncodeAll(input)
		decoded := tbl.DecodeAll(encoded)
		if !bytes.Equal(decoded, input) {
			t.Errorf("roundtrip failed: got %q, want %q", decoded, input)
		}
	}
}

func TestTable12Roundtrip(t *testing.T) {
	inputs := [][]byte{
		[]byte(""),
		[]byte("a"),
		[]byte("ab"),
		[]byte("abc"),
		[]byte("hello"),
		[]byte("the quick brown fox jumps over the lazy dog multiple times to generate patterns"),
		bytes.Repeat([]byte("pattern "), 100),
	}

	tbl := Train12(inputs)

	for _, input := range inputs {
		encoded := tbl.EncodeAll(input)
		decoded := tbl.DecodeAll(encoded)
		if !bytes.Equal(decoded, input) {
			t.Errorf("roundtrip failed for len=%d: got %q, want %q", len(input), decoded, input)
		}
	}
}

func TestTable12Serialize(t *testing.T) {
	data := [][]byte{
		[]byte("serialize test data with patterns patterns patterns"),
		[]byte("more patterns to learn symbols from the data"),
	}

	tbl1 := Train12(data)
	input := []byte("test input for serialization roundtrip")
	encoded1 := tbl1.EncodeAll(input)

	// Serialize
	serialized, err := tbl1.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Deserialize
	var tbl2 Table12
	if err := tbl2.UnmarshalBinary(serialized); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Verify encoding produces same result
	encoded2 := tbl2.EncodeAll(input)
	if !bytes.Equal(encoded1, encoded2) {
		t.Error("encoded outputs differ after serialization roundtrip")
	}

	// Verify decoding works
	decoded := tbl2.DecodeAll(encoded1)
	if !bytes.Equal(decoded, input) {
		t.Errorf("decode failed: got %q, want %q", decoded, input)
	}
}

func TestTable12Compression(t *testing.T) {
	// Highly repetitive data should compress well
	data := bytes.Repeat([]byte("the quick brown fox jumps "), 1000)
	inputs := [][]byte{data}

	tbl := Train12(inputs)
	encoded := tbl.EncodeAll(data)

	ratio := float64(len(encoded)) / float64(len(data))
	if ratio > 0.8 {
		t.Errorf("compression ratio too high: %.2f (expected < 0.8)", ratio)
	}

	decoded := tbl.DecodeAll(encoded)
	if !bytes.Equal(decoded, data) {
		t.Error("roundtrip failed")
	}
}

func TestTable12AllBytes(t *testing.T) {
	// Test all byte values
	var allBytes [256]byte
	for i := range 256 {
		allBytes[i] = byte(i)
	}
	input := allBytes[:]

	tbl := Train12([][]byte{input})
	encoded := tbl.EncodeAll(input)
	decoded := tbl.DecodeAll(encoded)

	if !bytes.Equal(decoded, input) {
		t.Error("all-bytes roundtrip failed")
	}
}

func TestTable12Strings(t *testing.T) {
	inputs := []string{
		"hello world",
		"the quick brown fox",
		"testing string interface",
	}

	tbl := Train12Strings(inputs)
	if tbl.nSymbols == 0 {
		t.Fatal("expected some learned symbols")
	}

	for _, s := range inputs {
		encoded := tbl.EncodeAll([]byte(s))
		decoded := tbl.DecodeAll(encoded)
		if string(decoded) != s {
			t.Errorf("string roundtrip failed: got %q, want %q", decoded, s)
		}
	}
}

// TestTable12ReadFromMalformed verifies that Table12.ReadFrom rejects
// crafted inputs without panicking.
func TestTable12ReadFromMalformed(t *testing.T) {
	validTable := func() []byte {
		tbl := Train12([][]byte{[]byte("hello world hello")})
		data, _ := tbl.MarshalBinary()
		return data
	}

	t.Run("bad_version", func(t *testing.T) {
		data := validTable()
		data[0] = 0xFF // corrupt version
		var tbl Table12
		_, err := tbl.ReadFrom(bytes.NewReader(data))
		if err != ErrBadVersion {
			t.Fatalf("expected ErrBadVersion, got %v", err)
		}
	})

	t.Run("nSymbols_exceeds_max", func(t *testing.T) {
		data := validTable()
		// nSymbols is at bytes 8-9 (after 8-byte version header)
		// Set to 65535 which exceeds fsst12MaxSymbols (3840)
		data[8] = 0xFF
		data[9] = 0xFF
		var tbl Table12
		_, err := tbl.ReadFrom(bytes.NewReader(data))
		if err != ErrCorrupted {
			t.Fatalf("expected ErrCorrupted, got %v", err)
		}
	})

	t.Run("lenHisto_sum_mismatch", func(t *testing.T) {
		data := validTable()
		// lenHisto starts at byte 10, 8 bytes long.
		// Set all histo entries to 255 so sum far exceeds nSymbols.
		for i := 10; i < 18; i++ {
			data[i] = 255
		}
		var tbl Table12
		_, err := tbl.ReadFrom(bytes.NewReader(data))
		if err != ErrCorrupted {
			t.Fatalf("expected ErrCorrupted, got %v", err)
		}
	})

	t.Run("truncated_input", func(t *testing.T) {
		data := validTable()
		var tbl Table12
		_, err := tbl.ReadFrom(bytes.NewReader(data[:8]))
		if err == nil {
			t.Fatalf("expected error on truncated input")
		}
	})
}

func table12MixedDecodeFixture(tb testing.TB) (*Table12, []byte, []byte, []byte) {
	tb.Helper()

	training := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog "), 1000)
	table := Train12([][]byte{training})
	trainingEncoded := table.EncodeAll(training)
	const prefixBytes = 9
	prefix := append([]byte(nil), trainingEncoded[:prefixBytes]...)
	prefixDecoded := table.DecodeAll(prefix)
	if len(prefixDecoded) <= len(prefix)*2 {
		tb.Fatalf("prefix does not exceed the 2x estimate: decoded=%d encoded=%d", len(prefixDecoded), len(prefix))
	}

	tail := make([]byte, 48*1024)
	state := uint64(0x9e3779b97f4a7c15)
	for i := range tail {
		state = state*6364136223846793005 + 1442695040888963407
		tail[i] = byte(state >> 56)
	}
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
	return table, input, encoded, prefix
}

func TestTable12DecodeSourceCapacityInvariant(t *testing.T) {
	table, input, encoded, _ := table12MixedDecodeFixture(t)

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
}

func TestTable12DecodeUnassignedTail(t *testing.T) {
	table, _, _, prefix := table12MixedDecodeFixture(t)

	unassigned := fsst12CodeMax - 1
	for unassigned >= fsst12CodeBase && table.decLen[unassigned] != 0 {
		unassigned--
	}
	if unassigned < fsst12CodeBase {
		t.Fatal("fixture table unexpectedly assigns every non-literal code")
	}

	want := table.DecodeAll(prefix)
	encoded := append(append([]byte(nil), prefix...), byte(unassigned), byte(unassigned>>8))
	var got []byte
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("DecodeAll panicked on an unassigned tail code: %v", recovered)
			}
		}()
		got = table.DecodeAll(encoded)
	}()
	if !bytes.Equal(got, want) {
		t.Fatalf("DecodeAll unassigned-tail output mismatch: got %d bytes, want %d", len(got), len(want))
	}
}

func BenchmarkTable12Encode(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog "), 1000)
	tbl := Train12([][]byte{data})

	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		_ = tbl.EncodeAll(data)
	}
}

func BenchmarkTable12Decode(b *testing.B) {
	data := bytes.Repeat([]byte("the quick brown fox jumps over the lazy dog "), 1000)
	tbl := Train12([][]byte{data})
	encoded := tbl.EncodeAll(data)

	b.ResetTimer()
	b.SetBytes(int64(len(data)))
	for i := 0; i < b.N; i++ {
		_ = tbl.DecodeAll(encoded)
	}
}
