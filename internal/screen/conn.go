package screen

import (
	"strings"
	"sync"

	"github.com/hexadecimil/hyprcage/internal/registry"
	"github.com/hexadecimil/hyprcage/internal/wl"
)

// conns caches one Wayland connection per screen. wl.Client is not safe for
// concurrent use; callers serialise (the MCP server runs one tool at a time).
var conns = struct {
	sync.Mutex
	m map[string]*wl.Client
}{m: map[string]*wl.Client{}}

// Open returns the Wayland connection to the screen's cage, opening it on
// first use and checking that the required protocols are there.
func Open(rec *registry.Screen) (*wl.Client, error) {
	conns.Lock()
	defer conns.Unlock()
	if cl, ok := conns.m[rec.Name]; ok {
		return cl, nil
	}
	if rec.InnerDisplay == "" {
		inner, err := registry.ReadInner(rec.Name)
		if err != nil {
			return nil, errf(CodeDead, "", "screen %s has no inner socket", rec.Name)
		}
		rec.InnerDisplay = inner["WAYLAND_DISPLAY"]
	}
	cl, err := wl.Connect(rec.InnerDisplay)
	if err != nil {
		return nil, errf(CodeDead, "hyprcage destroy "+rec.Name, "cannot connect to cage of %s: %v", rec.Name, err)
	}
	if missing := cl.Missing(); len(missing) > 0 {
		cl.Close()
		return nil, errf(CodeCage, "a newer cage is needed", "cage lacks %s", strings.Join(missing, ", "))
	}
	conns.m[rec.Name] = cl
	return cl, nil
}

// CloseConn drops the cached connection of a screen, if any.
func CloseConn(name string) {
	conns.Lock()
	defer conns.Unlock()
	if cl, ok := conns.m[name]; ok {
		cl.Close()
		delete(conns.m, name)
	}
}

// CloseAll drops every cached connection.
func CloseAll() {
	conns.Lock()
	defer conns.Unlock()
	for name, cl := range conns.m {
		cl.Close()
		delete(conns.m, name)
	}
}
