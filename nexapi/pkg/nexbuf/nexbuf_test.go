package nexbuf

import (
	"math"
	"testing"
)

func TestUint8RoundTrip(t *testing.T) {
	buf := NewBuffer(16)
	buf.WriteUint8(0)
	buf.WriteUint8(127)
	buf.WriteUint8(255)

	r := NewReader(buf.Bytes())
	for _, want := range []uint8{0, 127, 255} {
		got, err := r.ReadUint8()
		if err != nil {
			t.Fatalf("ReadUint8: %v", err)
		}
		if got != want {
			t.Errorf("ReadUint8 = %d, want %d", got, want)
		}
	}
}

func TestUint16RoundTrip(t *testing.T) {
	buf := NewBuffer(16)
	values := []uint16{0, 1, 256, 0xFFFF}
	for _, v := range values {
		buf.WriteUint16(v)
	}

	r := NewReader(buf.Bytes())
	for _, want := range values {
		got, err := r.ReadUint16()
		if err != nil {
			t.Fatalf("ReadUint16: %v", err)
		}
		if got != want {
			t.Errorf("ReadUint16 = %d, want %d", got, want)
		}
	}
}

func TestUint32RoundTrip(t *testing.T) {
	buf := NewBuffer(16)
	values := []uint32{0, 1, 1000000, 0xFFFFFFFF}
	for _, v := range values {
		buf.WriteUint32(v)
	}

	r := NewReader(buf.Bytes())
	for _, want := range values {
		got, err := r.ReadUint32()
		if err != nil {
			t.Fatalf("ReadUint32: %v", err)
		}
		if got != want {
			t.Errorf("ReadUint32 = %d, want %d", got, want)
		}
	}
}

func TestUint64RoundTrip(t *testing.T) {
	buf := NewBuffer(16)
	values := []uint64{0, 1, 1 << 40, 0xFFFFFFFFFFFFFFFF}
	for _, v := range values {
		buf.WriteUint64(v)
	}

	r := NewReader(buf.Bytes())
	for _, want := range values {
		got, err := r.ReadUint64()
		if err != nil {
			t.Fatalf("ReadUint64: %v", err)
		}
		if got != want {
			t.Errorf("ReadUint64 = %d, want %d", got, want)
		}
	}
}

func TestFloat32RoundTrip(t *testing.T) {
	buf := NewBuffer(16)
	values := []float32{0, 1.5, -3.14, math.MaxFloat32, math.SmallestNonzeroFloat32}
	for _, v := range values {
		buf.WriteFloat32(v)
	}

	r := NewReader(buf.Bytes())
	for _, want := range values {
		got, err := r.ReadFloat32()
		if err != nil {
			t.Fatalf("ReadFloat32: %v", err)
		}
		if got != want {
			t.Errorf("ReadFloat32 = %v, want %v", got, want)
		}
	}
}

func TestFloat64RoundTrip(t *testing.T) {
	buf := NewBuffer(16)
	values := []float64{0, 1.5, -3.14159265358979, math.MaxFloat64, math.SmallestNonzeroFloat64}
	for _, v := range values {
		buf.WriteFloat64(v)
	}

	r := NewReader(buf.Bytes())
	for _, want := range values {
		got, err := r.ReadFloat64()
		if err != nil {
			t.Fatalf("ReadFloat64: %v", err)
		}
		if got != want {
			t.Errorf("ReadFloat64 = %v, want %v", got, want)
		}
	}
}

func TestStringRoundTrip(t *testing.T) {
	buf := NewBuffer(64)
	values := []string{"", "hello", "Hello, World!", "unicode: \u00e4\u00f6\u00fc\u00df\u2603"}
	for _, v := range values {
		buf.WriteString(v)
	}

	r := NewReader(buf.Bytes())
	for _, want := range values {
		got, err := r.ReadString()
		if err != nil {
			t.Fatalf("ReadString: %v", err)
		}
		if got != want {
			t.Errorf("ReadString = %q, want %q", got, want)
		}
	}
}

func TestBytesRoundTrip(t *testing.T) {
	buf := NewBuffer(64)
	values := [][]byte{{}, {0x00}, {0xDE, 0xAD, 0xBE, 0xEF}, make([]byte, 256)}
	for i := range values[3] {
		values[3][i] = byte(i)
	}
	for _, v := range values {
		buf.WriteBytes(v)
	}

	r := NewReader(buf.Bytes())
	for i, want := range values {
		got, err := r.ReadBytes()
		if err != nil {
			t.Fatalf("ReadBytes[%d]: %v", i, err)
		}
		if len(got) != len(want) {
			t.Errorf("ReadBytes[%d] len = %d, want %d", i, len(got), len(want))
			continue
		}
		for j := range want {
			if got[j] != want[j] {
				t.Errorf("ReadBytes[%d][%d] = 0x%02x, want 0x%02x", i, j, got[j], want[j])
				break
			}
		}
	}
}

func TestListRoundTrip(t *testing.T) {
	buf := NewBuffer(64)
	// Write a list of 3 uint32 elements.
	buf.WriteList(3)
	buf.WriteUint32(10)
	buf.WriteUint32(20)
	buf.WriteUint32(30)

	r := NewReader(buf.Bytes())
	count, err := r.ReadList()
	if err != nil {
		t.Fatalf("ReadList: %v", err)
	}
	if count != 3 {
		t.Fatalf("ReadList count = %d, want 3", count)
	}
	expected := []uint32{10, 20, 30}
	for i := uint32(0); i < count; i++ {
		got, err := r.ReadUint32()
		if err != nil {
			t.Fatalf("ReadUint32[%d]: %v", i, err)
		}
		if got != expected[i] {
			t.Errorf("list[%d] = %d, want %d", i, got, expected[i])
		}
	}
}

func TestMapRoundTrip(t *testing.T) {
	buf := NewBuffer(64)
	entries := map[string]string{"key1": "val1", "key2": "val2"}
	buf.WriteMapLen(uint32(len(entries)))
	// Write in a deterministic order for the test.
	buf.WriteString("key1")
	buf.WriteString("val1")
	buf.WriteString("key2")
	buf.WriteString("val2")

	r := NewReader(buf.Bytes())
	count, err := r.ReadMapLen()
	if err != nil {
		t.Fatalf("ReadMapLen: %v", err)
	}
	if count != 2 {
		t.Fatalf("ReadMapLen = %d, want 2", count)
	}
	got := make(map[string]string, count)
	for i := uint32(0); i < count; i++ {
		k, err := r.ReadString()
		if err != nil {
			t.Fatalf("ReadString key: %v", err)
		}
		v, err := r.ReadString()
		if err != nil {
			t.Fatalf("ReadString val: %v", err)
		}
		got[k] = v
	}
	for k, want := range entries {
		if got[k] != want {
			t.Errorf("map[%q] = %q, want %q", k, got[k], want)
		}
	}
}

func TestReaderShortBuffer(t *testing.T) {
	r := NewReader([]byte{0x01}) // only 1 byte
	_, err := r.ReadUint32()
	if err != ErrShortBuffer {
		t.Errorf("expected ErrShortBuffer, got %v", err)
	}
}

func TestBufferGrowth(t *testing.T) {
	buf := NewBuffer(1) // tiny initial capacity
	for i := 0; i < 1000; i++ {
		buf.WriteUint32(uint32(i))
	}
	if buf.Len() != 4000 {
		t.Errorf("buf.Len() = %d, want 4000", buf.Len())
	}

	r := NewReader(buf.Bytes())
	for i := 0; i < 1000; i++ {
		got, err := r.ReadUint32()
		if err != nil {
			t.Fatalf("ReadUint32[%d]: %v", i, err)
		}
		if got != uint32(i) {
			t.Errorf("ReadUint32[%d] = %d, want %d", i, got, i)
		}
	}
}

func TestBufferReset(t *testing.T) {
	buf := NewBuffer(16)
	buf.WriteUint32(42)
	if buf.Len() != 4 {
		t.Fatalf("before reset: Len = %d", buf.Len())
	}
	buf.Reset()
	if buf.Len() != 0 {
		t.Fatalf("after reset: Len = %d", buf.Len())
	}
	buf.WriteUint32(99)
	r := NewReader(buf.Bytes())
	got, err := r.ReadUint32()
	if err != nil {
		t.Fatalf("ReadUint32 after reset: %v", err)
	}
	if got != 99 {
		t.Errorf("got %d, want 99", got)
	}
}
