package screen

import (
	"errors"
	"testing"

	"github.com/hexadecimil/hyprcage/internal/hypr"
	"github.com/hexadecimil/hyprcage/internal/registry"
	"github.com/hexadecimil/hyprcage/internal/session"
)

func TestValidateName(t *testing.T) {
	for _, ok := range []string{"hc-1a2b3c", "a", "test-screen-01"} {
		if err := ValidateName(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "Hc-1", "a b", `a"b`, "a;b", "x/y", "0123456789012345678901234567890123"} {
		err := ValidateName(bad)
		var se *Error
		if !errors.As(err, &se) || se.Code != CodeInvalidName {
			t.Errorf("%q should be invalid_name, got %v", bad, err)
		}
	}
}

func TestNewName(t *testing.T) {
	n := NewName("hc-")
	if err := ValidateName(n); err != nil || len(n) != 9 {
		t.Errorf("NewName = %q: %v", n, err)
	}
	if NewName("hc-") == NewName("hc-") {
		t.Error("names should differ")
	}
}

func TestOrphanScreenName(t *testing.T) {
	cases := map[string]string{
		"hyprcage-hc_1a2b3c.slice":            "hc-1a2b3c",
		"hyprcage-hc_1a2b3c-cage.scope":       "hc-1a2b3c",
		"hyprcage-hc_1a2b3c-mirror.scope":     "hc-1a2b3c",
		"hyprcage-hc_1a2b3c-app-4242-7.scope": "hc-1a2b3c",
		"hyprcage-gc-hc_1a2b3c.timer":         "hc-1a2b3c",
		"hyprcage-gc-hc_1a2b3c.service":       "hc-1a2b3c",
		"app-Hyprland-firefox-1.scope":        "",
	}
	for in, want := range cases {
		if got := orphanScreenName(in); got != want {
			t.Errorf("orphanScreenName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFarPosition(t *testing.T) {
	mons := []hypr.Monitor{{Name: "eDP-1", X: 0, Width: 1920}, {Name: "HDMI-A-1", X: 1920, Width: 1920}}
	x, y := farPosition(mons, 1280)
	if x != -farGap-1280 || y != 0 {
		t.Errorf("farPosition = %d,%d", x, y)
	}
	// A monitor already at a negative x pushes the output further left.
	mons = append(mons, hypr.Monitor{Name: "hc-a", X: -farGap - 1280, Width: 1280})
	if x, _ := farPosition(mons, 1280); x != -2*farGap-2*1280 {
		t.Errorf("second farPosition = %d", x)
	}
}

func TestFreeWorkspace(t *testing.T) {
	used := map[int]bool{11: true, 12: true, 6: true}
	if got := freeWorkspace(used, 11, 99); got != 13 {
		t.Errorf("app ws = %d, want 13", got)
	}
	if got := freeWorkspace(used, 6, 9); got != 7 {
		t.Errorf("mirror ws = %d, want 7", got)
	}
	if got := freeWorkspace(map[int]bool{6: true, 7: true, 8: true, 9: true}, 6, 9); got != 0 {
		t.Errorf("full range should give 0, got %d", got)
	}
}

func TestCheckOwner(t *testing.T) {
	rec := &registry.Screen{Name: "hc-a", Owner: registry.Owner{SessionID: "s1", PID: 10, PIDStart: 5}}
	if err := CheckOwner(rec, session.Identity{SessionID: "s1", PID: 99}); err != nil {
		t.Errorf("same session id must own: %v", err)
	}
	if err := CheckOwner(rec, session.Identity{SessionID: "s2", PID: 10, PIDStart: 5}); err == nil {
		t.Error("different session id must not own, even with the same pid")
	}
	if err := CheckOwner(rec, session.Identity{PID: 10, PIDStart: 5}); err != nil {
		t.Errorf("anonymous caller with the same pid must own: %v", err)
	}
	if err := CheckOwner(rec, session.Identity{PID: 10, PIDStart: 6}); err == nil {
		t.Error("reused pid must not own")
	}
	var se *Error
	if err := CheckOwner(rec, session.Identity{PID: 11}); !errors.As(err, &se) || se.Code != CodeNotOwner {
		t.Errorf("want not_owner, got %v", err)
	}
}

func TestPrefixed(t *testing.T) {
	cases := map[string]string{"dt": "hc-dt", "hc-dt": "hc-dt", "hc-": "hc-"}
	for in, want := range cases {
		if got := Prefixed(in, "hc-"); got != want {
			t.Errorf("Prefixed(%q) = %q, want %q", in, got, want)
		}
	}
	if got := Prefixed("dt", ""); got != "dt" {
		t.Errorf("an empty prefix must leave the name alone, got %q", got)
	}
}

func TestIsAgentOutput(t *testing.T) {
	known := map[string]bool{"dt": true} // a screen recorded before Prefixed
	for _, agent := range []string{"hc-1a2b3c", "hc-dt", "dt"} {
		if !isAgentOutput(agent, "hc-", known) {
			t.Errorf("%q is an agent output", agent)
		}
	}
	for _, human := range []string{"eDP-1", "HDMI-A-1", "DP-3"} {
		if isAgentOutput(human, "hc-", known) {
			t.Errorf("%q is one of the human's monitors", human)
		}
	}
}

func TestMirrorWanted(t *testing.T) {
	yes, no := true, false
	cases := []struct {
		want       *bool
		configured bool
		result     bool
	}{
		{nil, true, true},   // no opinion: the configuration decides
		{nil, false, false}, //
		{&no, true, false},  // the caller may drop a mirror the human asks for
		{&yes, false, true}, // and open one the human does not ask for
		{&yes, true, true},  //
		{&no, false, false}, //
	}
	for _, c := range cases {
		if got := mirrorWanted(c.want, c.configured); got != c.result {
			t.Errorf("mirrorWanted(%v, %v) = %v, want %v", c.want, c.configured, got, c.result)
		}
	}
}
