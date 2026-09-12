package registry

import (
	"os"
	"testing"
	"time"
)

func TestSaveLoadListDelete(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if l, err := List(); err != nil || len(l) != 0 {
		t.Fatalf("empty registry: %v, %v", l, err)
	}
	s := &Screen{Name: "hc-1a2b3c", CreatedAt: time.Now(), State: "ready", Width: 1280, Height: 800, WorkspaceApp: 11, Owner: Owner{PID: 1, Client: "test"}}
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	got, err := Load("hc-1a2b3c")
	if err != nil || got.Width != 1280 || got.Owner.Client != "test" {
		t.Fatalf("Load: %+v, %v", got, err)
	}
	if _, err := Load("nope"); err != ErrNotFound {
		t.Errorf("want ErrNotFound, got %v", err)
	}
	l, err := List()
	if err != nil || len(l) != 1 || l[0].Name != "hc-1a2b3c" {
		t.Fatalf("List: %v, %v", l, err)
	}
	fi, _ := os.Stat(Path("hc-1a2b3c"))
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("record perm %v, want 0600", fi.Mode().Perm())
	}
	if err := Delete("hc-1a2b3c"); err != nil {
		t.Fatal(err)
	}
	if err := Delete("hc-1a2b3c"); err != nil {
		t.Errorf("deleting twice must be fine: %v", err)
	}
}

func TestHeartbeat(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if err := Save(&Screen{Name: "hc-a"}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(Path("hc-a"), old, old); err != nil {
		t.Fatal(err)
	}
	if err := Touch("hc-a"); err != nil {
		t.Fatal(err)
	}
	hb, err := Heartbeat("hc-a")
	if err != nil || time.Since(hb) > time.Minute {
		t.Errorf("heartbeat not refreshed: %v, %v", hb, err)
	}
}

func TestReadInner(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if _, err := EnsureDir(); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(InnerPath("hc-a"), []byte("WAYLAND_DISPLAY=wayland-2\nDISPLAY=:3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m, err := ReadInner("hc-a")
	if err != nil || m["WAYLAND_DISPLAY"] != "wayland-2" || m["DISPLAY"] != ":3" {
		t.Fatalf("ReadInner: %v, %v", m, err)
	}
}
