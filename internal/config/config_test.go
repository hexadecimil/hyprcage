package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFileMissingGivesDefaults(t *testing.T) {
	cfg, err := LoadFile(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil || cfg.Loaded || cfg != func() Config { d := Default(); d.Path = cfg.Path; return d }() {
		t.Fatalf("missing file: err=%v loaded=%v", err, cfg.Loaded)
	}
}

func TestLoadFileOverrides(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(`
[screen]
width = 1024
height = 640
max_width = 2560
max_height = 1440
max_per_session = 2
mirror = false
notify = false

[workspaces]
agent = [20, 30]
mirror = [7, 8]

[lifecycle]
safety_timer = "30m"

[cage]
renderer = "Pixman"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Loaded || cfg.Path != p {
		t.Errorf("loaded=%v path=%s", cfg.Loaded, cfg.Path)
	}
	want := Default()
	want.Path = p
	want.Loaded = true
	want.DefaultWidth, want.DefaultHeight = 1024, 640
	want.MaxWidth, want.MaxHeight = 2560, 1440
	want.MaxPerSession = 2
	want.MirrorEnabled, want.Notify = false, false
	want.WorkspaceMin, want.WorkspaceMax = 20, 30
	want.MirrorMin, want.MirrorMax = 7, 8
	want.SafetyTimer = 30 * time.Minute
	want.Renderer = "pixman"
	if cfg != want {
		t.Errorf("got  %+v\nwant %+v", cfg, want)
	}
}

func TestApplyRejects(t *testing.T) {
	cases := map[string]string{
		`[screen]` + "\n" + `widht = 10`:               "unknown key",
		`[screen]` + "\n" + `width = 100`:              "below 320x240",
		`[screen]` + "\n" + `max_width = 800`:          "below the default size",
		`[screen]` + "\n" + `max_per_session = -1`:     "negative",
		`[workspaces]` + "\n" + `agent = [5]`:          "expected [min, max]",
		`[workspaces]` + "\n" + `agent = [0, 9]`:       ">= 1",
		`[workspaces]` + "\n" + `agent = [30, 20]`:     ">= 1",
		`[workspaces]` + "\n" + `agent = [6, 20]`:      "overlap",
		`[workspaces]` + "\n" + `mirror = [11, 12]`:    "overlap",
		`[lifecycle]` + "\n" + `safety_timer = "soon"`: "safety_timer",
		`[lifecycle]` + "\n" + `safety_timer = "10s"`:  "below 1m",
		`[cage]` + "\n" + `renderer = "vulkan"`:        "not auto, gles or pixman",
		`screen = 3`:                                   "",
	}
	for text, want := range cases {
		cfg := Default()
		err := Apply(&cfg, text)
		if err == nil {
			t.Errorf("%q: expected an error", text)
			continue
		}
		if want != "" && !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %q, want it to mention %q", text, err, want)
		}
	}
}

func TestPathHonoursXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/tmp/x")
	if got := Path(); got != "/tmp/x/hyprcage/config.toml" {
		t.Errorf("Path() = %s", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "")
	if got := Path(); !strings.HasSuffix(got, "/.config/hyprcage/config.toml") {
		t.Errorf("Path() = %s", got)
	}
}
