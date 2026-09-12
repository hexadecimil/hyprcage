package screen

import (
	"errors"
	"strings"

	"github.com/hexadecimil/hyprcage/internal/registry"
	"github.com/hexadecimil/hyprcage/internal/session"
)

// Get loads a record and checks that its output and cage are still there.
func Get(c *Ctx, name string) (*registry.Screen, error) {
	rec, err := registry.Load(name)
	if errors.Is(err, registry.ErrNotFound) {
		return nil, errf(CodeNotFound, "hyprcage list --all", "no screen named %s", name)
	}
	if err != nil {
		return nil, err
	}
	if !hasMonitor(c.Hypr, rec.Name) || (rec.CagePID > 0 && !session.PIDAlive(rec.CagePID, 0)) {
		return rec, errf(CodeDead, "hyprcage destroy "+name+" then create a new one", "screen %s is dead: its output or its cage is gone", name)
	}
	return rec, nil
}

// Resolve implements cahier F14: an empty name means the session's only screen.
func Resolve(c *Ctx, name string, id session.Identity) (*registry.Screen, error) {
	if name != "" {
		return Get(c, name)
	}
	recs, err := registry.List()
	if err != nil {
		return nil, err
	}
	var mine []*registry.Screen
	for _, r := range recs {
		if CheckOwner(r, id) == nil {
			mine = append(mine, r)
		}
	}
	switch len(mine) {
	case 0:
		return nil, errf(CodeNotFound, "hyprcage create", "this session has no screen")
	case 1:
		return Get(c, mine[0].Name)
	}
	names := make([]string, len(mine))
	for i, r := range mine {
		names[i] = r.Name
	}
	return nil, errf(CodeNotFound, "name the screen", "this session has several screens: %s", strings.Join(names, ", "))
}
