package screen

import (
	"regexp"
	"strings"
	"time"

	"github.com/hexadecimil/hyprcage/internal/lock"
	"github.com/hexadecimil/hyprcage/internal/registry"
	"github.com/hexadecimil/hyprcage/internal/session"
	"github.com/hexadecimil/hyprcage/internal/sysd"
)

// GCOptions selects what gc destroys beyond dead sessions.
type GCOptions struct {
	All     bool   // every screen
	Session string // the screens of this session id
}

// GCReport lists what gc did.
type GCReport struct {
	Destroyed []string `json:"destroyed"`
	Orphans   []string `json:"orphans"`
	Errors    []string `json:"errors"`
}

var appSuffixRE = regexp.MustCompile(`-app-\d+(-\d+)?$`)

// GC implements cahier §5.3 M4: screens of dead sessions, then units and
// outputs that have no record.
func GC(c *Ctx, opts GCOptions) (GCReport, error) {
	var rep GCReport
	release, err := lock.Acquire(10 * time.Second)
	if err != nil {
		return rep, err
	}
	defer release()

	recs, err := registry.List()
	if err != nil {
		return rep, err
	}
	known := map[string]bool{}
	for _, rec := range recs {
		known[rec.Name] = true
		hb, _ := registry.Heartbeat(rec.Name)
		dead := !session.IsAlive(rec.Owner, hb, c.Cfg.SessionGrace)
		// A screen whose cage crashed is useless whoever owns it: reap it too.
		cageDead := rec.State == "ready" && rec.CagePID > 0 && !session.PIDAlive(rec.CagePID, 0)
		if opts.All || (opts.Session != "" && rec.Owner.SessionID == opts.Session) || dead || cageDead {
			if err := Destroy(c, rec); err != nil {
				rep.Errors = append(rep.Errors, rec.Name+": "+err.Error())
				continue
			}
			rep.Destroyed = append(rep.Destroyed, rec.Name)
		}
	}

	if sysd.Available() {
		units, _ := sysd.ListUnits("hyprcage-*")
		for _, u := range units {
			name := orphanScreenName(u)
			if name == "" || known[name] {
				continue
			}
			_ = sysd.StopUnit(u)
			rep.Orphans = append(rep.Orphans, u)
		}
	}
	if mons, err := c.Hypr.Monitors(); err == nil {
		for _, m := range mons {
			if !strings.HasPrefix(m.Name, c.Cfg.OutputPrefix) || known[m.Name] {
				continue
			}
			before := captureHuman(c.Hypr, c.Cfg.OutputPrefix)
			if err := c.Hypr.RemoveOutput(m.Name); err == nil {
				restoreHuman(c.Hypr, c.Driver, before, c.Cfg.OutputPrefix)
				rep.Orphans = append(rep.Orphans, "output "+m.Name)
			}
		}
	}
	return rep, nil
}

// orphanScreenName extracts the screen name from a unit name such as
// hyprcage-<name>.slice, hyprcage-<name>-cage.scope, hyprcage-<name>-app-N.scope
// or hyprcage-gc-<name>.timer.
func orphanScreenName(unit string) string {
	base := strings.TrimPrefix(unit, "hyprcage-")
	if base == unit {
		return ""
	}
	if i := strings.LastIndex(base, "."); i >= 0 {
		base = base[:i]
	}
	if strings.HasPrefix(base, "gc-") {
		return sysd.ScreenFromUnit(strings.TrimPrefix(base, "gc-"))
	}
	base = appSuffixRE.ReplaceAllString(base, "")
	base = strings.TrimSuffix(base, "-cage")
	base = strings.TrimSuffix(base, "-mirror")
	return sysd.ScreenFromUnit(base)
}

// heartbeatAge is a helper for callers reporting liveness.
func heartbeatAge(name string) time.Duration {
	hb, err := registry.Heartbeat(name)
	if err != nil {
		return 0
	}
	return time.Since(hb)
}
