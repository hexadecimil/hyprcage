// Package config holds the settings of cahier §6.3: built-in defaults,
// overridden by ~/.config/hyprcage/config.toml when that file exists.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config is the effective configuration.
type Config struct {
	OutputPrefix  string
	DefaultWidth  int
	DefaultHeight int
	MaxWidth      int // largest screen size accepted by create
	MaxHeight     int
	MaxPerSession int // screens one session may hold at once; 0 = no limit
	RefreshHz     int
	WorkspaceMin  int // agent workspaces, one per screen
	WorkspaceMax  int
	MirrorEnabled bool
	MirrorMin     int // workspaces the mirror windows land on
	MirrorMax     int
	ProtectedMin  int
	ProtectedMax  int
	Notify        bool
	Renderer      string // cage renderer: auto, gles or pixman (HYPRCAGE_RENDERER wins)
	SessionGrace  time.Duration
	SafetyTimer   time.Duration
	WindowTimeout time.Duration

	ShotMaxSide     int     // hard cap per capture side (cahier F3)
	ShotMaxBytes    int     // encoded size budget
	StableThreshold float64 // wait --stable: max percent of changed pixels (8x reduced)

	Path   string // the config file looked at
	Loaded bool   // the file existed and was applied
}

// Default returns the built-in defaults.
func Default() Config {
	return Config{
		OutputPrefix:  "hc-",
		DefaultWidth:  1280,
		DefaultHeight: 800,
		MaxWidth:      3840,
		MaxHeight:     2160,
		MaxPerSession: 4,
		RefreshHz:     60,
		WorkspaceMin:  11,
		WorkspaceMax:  99,
		MirrorEnabled: true,
		MirrorMin:     6,
		MirrorMax:     9,
		ProtectedMin:  1,
		ProtectedMax:  5,
		Notify:        true,
		Renderer:      "auto",
		SessionGrace:  120 * time.Second,
		SafetyTimer:   15 * time.Minute,
		WindowTimeout: 10 * time.Second,

		ShotMaxSide:     2000,
		ShotMaxBytes:    1 << 20,
		StableThreshold: 0.02,
	}
}

// Path is the config file: $XDG_CONFIG_HOME/hyprcage/config.toml, that is
// ~/.config/hyprcage/config.toml by default.
func Path() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "hyprcage", "config.toml")
}

// Load returns the defaults overridden by the config file when it exists.
// A file that cannot be read or fails validation is an error: running on
// the defaults without a word would hide a typo.
func Load() (Config, error) { return LoadFile(Path()) }

// LoadFile is Load for an explicit path; a missing file yields the defaults.
func LoadFile(path string) (Config, error) {
	cfg := Default()
	cfg.Path = path
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	if err := Apply(&cfg, string(data)); err != nil {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	cfg.Loaded = true
	return cfg, nil
}

// Fallback is Load for the background paths (gc, the session hooks, the
// watcher): a broken file is reported on stderr and the defaults apply, so
// that the safety nets keep working whatever the file says.
func Fallback() Config {
	cfg, err := Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "hyprcage: %v; using the defaults\n", err)
		d := Default()
		d.Path = cfg.Path
		return d
	}
	return cfg
}

// file is the shape of config.toml. Every key is optional; unknown keys
// are rejected.
type file struct {
	Screen struct {
		Width         *int  `toml:"width"`
		Height        *int  `toml:"height"`
		MaxWidth      *int  `toml:"max_width"`
		MaxHeight     *int  `toml:"max_height"`
		MaxPerSession *int  `toml:"max_per_session"`
		Mirror        *bool `toml:"mirror"`
		Notify        *bool `toml:"notify"`
	} `toml:"screen"`
	Workspaces struct {
		Agent  []int `toml:"agent"`
		Mirror []int `toml:"mirror"`
	} `toml:"workspaces"`
	Lifecycle struct {
		SafetyTimer string `toml:"safety_timer"`
	} `toml:"lifecycle"`
	Cage struct {
		Renderer string `toml:"renderer"`
	} `toml:"cage"`
}

// Apply overrides cfg with the TOML text and validates the result.
func Apply(cfg *Config, text string) error {
	var f file
	md, err := toml.Decode(text, &f)
	if err != nil {
		return err
	}
	if un := md.Undecoded(); len(un) > 0 {
		names := make([]string, len(un))
		for i, k := range un {
			names[i] = k.String()
		}
		return fmt.Errorf("unknown key(s): %s", strings.Join(names, ", "))
	}
	setInt := func(dst *int, src *int) {
		if src != nil {
			*dst = *src
		}
	}
	setInt(&cfg.DefaultWidth, f.Screen.Width)
	setInt(&cfg.DefaultHeight, f.Screen.Height)
	setInt(&cfg.MaxWidth, f.Screen.MaxWidth)
	setInt(&cfg.MaxHeight, f.Screen.MaxHeight)
	setInt(&cfg.MaxPerSession, f.Screen.MaxPerSession)
	if f.Screen.Mirror != nil {
		cfg.MirrorEnabled = *f.Screen.Mirror
	}
	if f.Screen.Notify != nil {
		cfg.Notify = *f.Screen.Notify
	}
	if f.Workspaces.Agent != nil {
		if cfg.WorkspaceMin, cfg.WorkspaceMax, err = span("workspaces.agent", f.Workspaces.Agent); err != nil {
			return err
		}
	}
	if f.Workspaces.Mirror != nil {
		if cfg.MirrorMin, cfg.MirrorMax, err = span("workspaces.mirror", f.Workspaces.Mirror); err != nil {
			return err
		}
	}
	if f.Lifecycle.SafetyTimer != "" {
		d, err := time.ParseDuration(f.Lifecycle.SafetyTimer)
		if err != nil {
			return fmt.Errorf("lifecycle.safety_timer: %w", err)
		}
		cfg.SafetyTimer = d
	}
	if f.Cage.Renderer != "" {
		cfg.Renderer = strings.ToLower(strings.TrimSpace(f.Cage.Renderer))
	}
	return cfg.Validate()
}

// span reads a [min, max] pair.
func span(key string, v []int) (int, int, error) {
	if len(v) != 2 {
		return 0, 0, fmt.Errorf("%s: expected [min, max], got %v", key, v)
	}
	return v[0], v[1], nil
}

// Validate checks the values against each other and against what Hyprland
// and cage accept.
func (c Config) Validate() error {
	switch {
	case c.DefaultWidth < 320 || c.DefaultHeight < 240:
		return fmt.Errorf("screen.width/height: %dx%d is below 320x240", c.DefaultWidth, c.DefaultHeight)
	case c.MaxWidth > 16384 || c.MaxHeight > 16384:
		return fmt.Errorf("screen.max_width/max_height: %dx%d is above 16384", c.MaxWidth, c.MaxHeight)
	case c.MaxWidth < c.DefaultWidth || c.MaxHeight < c.DefaultHeight:
		return fmt.Errorf("screen.max_width/max_height: %dx%d is below the default size %dx%d", c.MaxWidth, c.MaxHeight, c.DefaultWidth, c.DefaultHeight)
	case c.MaxPerSession < 0:
		return fmt.Errorf("screen.max_per_session: %d is negative (0 means no limit)", c.MaxPerSession)
	case c.WorkspaceMin < 1 || c.WorkspaceMin > c.WorkspaceMax:
		return fmt.Errorf("workspaces.agent: [%d, %d] is not a range of workspaces >= 1", c.WorkspaceMin, c.WorkspaceMax)
	case c.MirrorMin < 1 || c.MirrorMin > c.MirrorMax:
		return fmt.Errorf("workspaces.mirror: [%d, %d] is not a range of workspaces >= 1", c.MirrorMin, c.MirrorMax)
	case c.WorkspaceMin <= c.MirrorMax && c.MirrorMin <= c.WorkspaceMax:
		return fmt.Errorf("workspaces.agent [%d, %d] and workspaces.mirror [%d, %d] overlap", c.WorkspaceMin, c.WorkspaceMax, c.MirrorMin, c.MirrorMax)
	case c.SafetyTimer < time.Minute:
		return fmt.Errorf("lifecycle.safety_timer: %s is below 1m", c.SafetyTimer)
	}
	switch c.Renderer {
	case "auto", "gles", "pixman":
	default:
		return fmt.Errorf("cage.renderer: %q is not auto, gles or pixman", c.Renderer)
	}
	return nil
}
