package wl

import (
	"bytes"
	"encoding/binary"
	"reflect"
	"testing"
)

func TestPutU32AndDecode(t *testing.T) {
	b := putU32(nil, 0x01020304)
	if !bytes.Equal(b, []byte{0x04, 0x03, 0x02, 0x01}) {
		t.Fatalf("little endian expected, got % x", b)
	}
	d := &decoder{b: b}
	v, err := d.u32()
	if err != nil || v != 0x01020304 {
		t.Fatalf("u32 = %#x, %v", v, err)
	}
}

func TestIntAndUintSignedness(t *testing.T) {
	neg := int32(-7)
	b := putU32(nil, uint32(neg))
	if binary.LittleEndian.Uint32(b) != 0xfffffff9 {
		t.Fatalf("−7 encoded as % x", b)
	}
	args, err := decodeArgs([]argSpec{{Name: "v", Type: argInt}}, b, new([]int))
	if err != nil {
		t.Fatal(err)
	}
	if args[0].(int32) != -7 {
		t.Fatalf("decoded %v, want -7", args[0])
	}
	args, err = decodeArgs([]argSpec{{Name: "v", Type: argUint}}, b, new([]int))
	if err != nil {
		t.Fatal(err)
	}
	if args[0].(uint32) != 0xfffffff9 {
		t.Fatalf("decoded %v as uint", args[0])
	}
}

func TestFixed(t *testing.T) {
	cases := []struct {
		in   int
		wire int32
		f    float64
	}{
		{0, 0, 0},
		{15, 15 * 256, 15},
		{-15, -15 * 256, -15},
		{-1, -256, -1},
	}
	for _, c := range cases {
		got := fixedFromInt(c.in)
		if int32(got) != c.wire {
			t.Errorf("fixedFromInt(%d) = %d, want %d", c.in, int32(got), c.wire)
		}
		if got.float() != c.f {
			t.Errorf("fixed(%d).float() = %v, want %v", int32(got), got.float(), c.f)
		}
		b := putU32(nil, uint32(int32(got)))
		args, err := decodeArgs([]argSpec{{Name: "v", Type: argFixed}}, b, new([]int))
		if err != nil {
			t.Fatal(err)
		}
		if args[0].(fixed) != got {
			t.Errorf("fixed round trip: %v != %v", args[0], got)
		}
	}
}

func TestStringPadding(t *testing.T) {
	cases := []struct {
		s    string
		size int // total bytes: length word + payload + padding
	}{
		{"", 4 + 4},     // just the NUL, padded to 4
		{"a", 4 + 4},    // "a\0" padded to 4
		{"abc", 4 + 4},  // "abc\0" is already a multiple of 4
		{"abcd", 4 + 8}, // "abcd\0" padded to 8
		{"wl_shm", 4 + 8},
	}
	for _, c := range cases {
		b := putString(nil, c.s)
		if len(b) != c.size {
			t.Errorf("putString(%q) = %d bytes, want %d", c.s, len(b), c.size)
		}
		if len(b)%4 != 0 {
			t.Errorf("putString(%q) is not padded to 4", c.s)
		}
		if n := binary.LittleEndian.Uint32(b); int(n) != len(c.s)+1 {
			t.Errorf("putString(%q) length word = %d, want %d", c.s, n, len(c.s)+1)
		}
		d := &decoder{b: b}
		got, err := d.str()
		if err != nil {
			t.Fatal(err)
		}
		if got != c.s {
			t.Errorf("string round trip: %q != %q", got, c.s)
		}
		if d.off != len(b) {
			t.Errorf("decoder left %d bytes for %q", len(b)-d.off, c.s)
		}
	}
}

func TestArrayPadding(t *testing.T) {
	in := []byte{1, 2, 3, 4, 5}
	b := putArray(nil, in)
	if len(b) != 4+8 {
		t.Fatalf("putArray = %d bytes, want 12", len(b))
	}
	d := &decoder{b: b}
	got, err := d.array()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, in) {
		t.Fatalf("array round trip: % x != % x", got, in)
	}
	if d.off != len(b) {
		t.Fatalf("decoder left %d bytes", len(b)-d.off)
	}
}

func TestMessageHeader(t *testing.T) {
	m := &protoIfaces[ifaceWlDisplay].Requests[reqWlDisplaySync]
	buf, fds, err := encodeMessage(displayID, m, []any{uint32(2)})
	if err != nil {
		t.Fatal(err)
	}
	if len(fds) != 0 {
		t.Fatalf("unexpected file descriptors")
	}
	if len(buf) != 12 {
		t.Fatalf("sync message is %d bytes, want 12", len(buf))
	}
	if id := binary.LittleEndian.Uint32(buf[0:4]); id != 1 {
		t.Errorf("object id = %d, want 1", id)
	}
	word := binary.LittleEndian.Uint32(buf[4:8])
	if size := word >> 16; size != 12 {
		t.Errorf("size = %d, want 12", size)
	}
	if op := word & 0xffff; op != reqWlDisplaySync {
		t.Errorf("opcode = %d, want %d", op, reqWlDisplaySync)
	}
	if cb := binary.LittleEndian.Uint32(buf[8:12]); cb != 2 {
		t.Errorf("callback id = %d, want 2", cb)
	}
}

// TestMessageRoundTrip encodes a request with every awkward argument type and
// decodes it again through the generated signature.
func TestMessageRoundTrip(t *testing.T) {
	m := &protoIfaces[ifaceZwlrVirtualPointerV1].Requests[reqZwlrVirtualPointerV1AxisDiscrete]
	buf, _, err := encodeMessage(7, m, []any{uint32(4242), uint32(0), fixedFromInt(-15), int32(-1)})
	if err != nil {
		t.Fatal(err)
	}
	id, op, body := splitMessage(t, buf)
	if id != 7 || op != reqZwlrVirtualPointerV1AxisDiscrete {
		t.Fatalf("header: id %d op %d", id, op)
	}
	args, err := decodeArgs(m.Args, body, new([]int))
	if err != nil {
		t.Fatal(err)
	}
	want := []any{uint32(4242), uint32(0), fixedFromInt(-15), int32(-1)}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %#v, want %#v", args, want)
	}
}

// TestBindNewIDAny covers the new_id argument without interface, which puts an
// interface name and a version on the wire (wl_registry.bind).
func TestBindNewIDAny(t *testing.T) {
	m := &protoIfaces[ifaceWlRegistry].Requests[reqWlRegistryBind]
	nid := newIDAny{Interface: "wl_shm", Version: 1, ID: 9}
	buf, _, err := encodeMessage(2, m, []any{uint32(3), nid})
	if err != nil {
		t.Fatal(err)
	}
	_, _, body := splitMessage(t, buf)
	args, err := decodeArgs(m.Args, body, new([]int))
	if err != nil {
		t.Fatal(err)
	}
	if args[0].(uint32) != 3 {
		t.Fatalf("name = %v", args[0])
	}
	if got := args[1].(newIDAny); got != nid {
		t.Fatalf("new_id = %#v, want %#v", got, nid)
	}
}

func splitMessage(t *testing.T, buf []byte) (uint32, int, []byte) {
	t.Helper()
	if len(buf) < 8 {
		t.Fatalf("message shorter than a header")
	}
	id := binary.LittleEndian.Uint32(buf[0:4])
	word := binary.LittleEndian.Uint32(buf[4:8])
	size := int(word >> 16)
	if size != len(buf) {
		t.Fatalf("size %d, buffer %d", size, len(buf))
	}
	if size%4 != 0 {
		t.Fatalf("size %d is not a multiple of 4", size)
	}
	return id, int(word & 0xffff), buf[8:]
}

func TestSocketPathResolution(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	got, err := socketPath("wayland-1")
	if err != nil || got != "/run/user/1000/wayland-1" {
		t.Fatalf("socketPath(relative) = %q, %v", got, err)
	}
	got, err = socketPath("/tmp/sockets/cage-42")
	if err != nil || got != "/tmp/sockets/cage-42" {
		t.Fatalf("socketPath(absolute) = %q, %v", got, err)
	}
}
