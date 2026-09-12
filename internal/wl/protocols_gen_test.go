package wl

import "testing"

// TestGeneratedOpcodes pins the opcodes that matter, cross checked by hand
// against the XML declaration order. A wrong opcode is the classic invisible
// Wayland bug, so these values are asserted rather than trusted.
func TestGeneratedOpcodes(t *testing.T) {
	requests := []struct {
		iface, name string
		want        int
	}{
		{ifaceWlDisplay, "sync", 0},
		{ifaceWlDisplay, "get_registry", 1},
		{ifaceWlRegistry, "bind", 0},
		{ifaceZwlrVirtualPointerV1, "motion_absolute", 1},
		{ifaceZwlrVirtualPointerV1, "button", 2},
		{ifaceZwlrVirtualPointerV1, "frame", 4},
		{ifaceZwlrVirtualPointerV1, "axis_source", 5},
		{ifaceZwlrVirtualPointerV1, "axis_discrete", 7},
		{ifaceZwpVirtualKeyboardV1, "keymap", 0},
		{ifaceZwpVirtualKeyboardV1, "key", 1},
		{ifaceZwpVirtualKeyboardV1, "modifiers", 2},
		{ifaceZwlrScreencopyManagerV1, "capture_output", 0},
		{ifaceZwlrScreencopyFrameV1, "copy", 0},
		{ifaceWlShm, "create_pool", 0},
		{ifaceWlShmPool, "create_buffer", 0},
		{ifaceZwlrForeignToplevelHandleV1, "close", 5},
	}
	for _, c := range requests {
		spec, ok := protoIfaces[c.iface]
		if !ok {
			t.Fatalf("interface %s is not generated", c.iface)
		}
		got := -1
		for i := range spec.Requests {
			if spec.Requests[i].Name == c.name {
				got = spec.Requests[i].Opcode
				if got != i {
					t.Errorf("%s.%s: opcode %d at index %d", c.iface, c.name, got, i)
				}
			}
		}
		if got != c.want {
			t.Errorf("request %s.%s opcode = %d, want %d", c.iface, c.name, got, c.want)
		}
	}

	events := []struct {
		iface, name string
		want        int
	}{
		{ifaceWlDisplay, "error", 0},
		{ifaceWlDisplay, "delete_id", 1},
		{ifaceWlRegistry, "global", 0},
		{ifaceWlCallback, "done", 0},
		{ifaceWlOutput, "mode", 1},
		{ifaceWlOutput, "done", 2},
		{ifaceWlOutput, "scale", 3},
		{ifaceZwlrScreencopyFrameV1, "buffer", 0},
		{ifaceZwlrScreencopyFrameV1, "flags", 1},
		{ifaceZwlrScreencopyFrameV1, "ready", 2},
		{ifaceZwlrScreencopyFrameV1, "failed", 3},
		{ifaceZwlrScreencopyFrameV1, "buffer_done", 6},
		{ifaceZwlrForeignToplevelManagerV1, "toplevel", 0},
		{ifaceZwlrForeignToplevelHandleV1, "state", 4},
		{ifaceZwlrForeignToplevelHandleV1, "done", 5},
		{ifaceZwlrForeignToplevelHandleV1, "closed", 6},
	}
	for _, c := range events {
		spec := protoIfaces[c.iface]
		got := -1
		for i := range spec.Events {
			if spec.Events[i].Name == c.name {
				got = spec.Events[i].Opcode
				if got != i {
					t.Errorf("%s.%s: opcode %d at index %d", c.iface, c.name, got, i)
				}
			}
		}
		if got != c.want {
			t.Errorf("event %s.%s opcode = %d, want %d", c.iface, c.name, got, c.want)
		}
	}

	// The named constants must agree with the tables.
	if reqWlDisplaySync != 0 || reqWlDisplayGetRegistry != 1 || reqWlRegistryBind != 0 {
		t.Errorf("wl_display/wl_registry constants drifted")
	}
	if reqZwlrVirtualPointerV1MotionAbsolute != 1 || reqZwlrScreencopyFrameV1Copy != 0 {
		t.Errorf("input/screencopy request constants drifted")
	}
	if evtWlDisplayError != 0 || evtWlDisplayDeleteId != 1 {
		t.Errorf("wl_display event constants drifted")
	}
	if evtZwlrScreencopyFrameV1Buffer != 0 || evtZwlrScreencopyFrameV1Flags != 1 ||
		evtZwlrScreencopyFrameV1Ready != 2 || evtZwlrScreencopyFrameV1Failed != 3 {
		t.Errorf("screencopy event constants drifted")
	}
}

// TestGeneratedSignatures spot checks argument signatures: a wrong type shifts
// every following argument.
func TestGeneratedSignatures(t *testing.T) {
	m := protoIfaces[ifaceZwlrVirtualPointerV1].Requests[reqZwlrVirtualPointerV1MotionAbsolute]
	want := []argType{argUint, argUint, argUint, argUint, argUint}
	if len(m.Args) != len(want) {
		t.Fatalf("motion_absolute has %d arguments, want %d", len(m.Args), len(want))
	}
	for i, w := range want {
		if m.Args[i].Type != w {
			t.Errorf("motion_absolute arg %d (%s) type %d, want %d", i, m.Args[i].Name, m.Args[i].Type, w)
		}
	}
	km := protoIfaces[ifaceZwpVirtualKeyboardV1].Requests[reqZwpVirtualKeyboardV1Keymap]
	if len(km.Args) != 3 || km.Args[1].Type != argFD {
		t.Fatalf("keymap signature = %#v", km.Args)
	}
	bind := protoIfaces[ifaceWlRegistry].Requests[reqWlRegistryBind]
	if len(bind.Args) != 2 || bind.Args[1].Type != argNewID || bind.Args[1].Interface != "" {
		t.Fatalf("bind signature = %#v", bind.Args)
	}
	st := protoIfaces[ifaceZwlrForeignToplevelHandleV1].Events[evtZwlrForeignToplevelHandleV1State]
	if len(st.Args) != 1 || st.Args[0].Type != argArray {
		t.Fatalf("toplevel state signature = %#v", st.Args)
	}
}
