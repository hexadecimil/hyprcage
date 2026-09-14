package screen

import (
	"os"
	"syscall"
	"time"

	"github.com/hexadecimil/hyprcage/internal/lock"
	"github.com/hexadecimil/hyprcage/internal/notify"
	"github.com/hexadecimil/hyprcage/internal/registry"
	"github.com/hexadecimil/hyprcage/internal/session"
	"github.com/hexadecimil/hyprcage/internal/sysd"
)

// Destroy closes a screen: its applications, its mirror, then cage, in that
// order so that the applications see their SIGTERM before their compositor
// is gone. Nothing here touches the human's compositor: the screen's output
// was cage's own. Ownership is checked by the caller.
func Destroy(c *Ctx, rec *registry.Screen) error {
	if rec.Version < RecordVersion {
		return destroyLegacy(c, rec)
	}
	release, err := lock.Acquire(30 * time.Second)
	if err != nil {
		return err
	}
	defer release()

	CloseConn(rec.Name)
	StopMirror(rec.Name)
	if sysd.Available() {
		sysd.StopScreen(rec.Name) // applications, mirror, cage, slice, reset-failed
		sysd.DisarmTimer(sysd.GCTimerName(rec.Name))
	}
	// With or without systemd, whatever still carries the screen's name
	// gets a SIGTERM, then a SIGKILL if it lingers.
	killScreenProcesses(rec.Name, syscall.SIGTERM)
	if !waitNoProcesses(rec.Name, 3*time.Second) {
		killScreenProcesses(rec.Name, syscall.SIGKILL)
		waitNoProcesses(rec.Name, time.Second)
	}
	_ = os.Remove(registry.InnerPath(rec.Name))
	if err := registry.Delete(rec.Name); err != nil {
		return err
	}
	if c.Cfg.Notify {
		notify.Send("Agent screen closed", rec.Name)
	}
	return nil
}

// Alive reports whether a screen's cage still runs.
func Alive(rec *registry.Screen) bool {
	if rec.CagePID <= 0 {
		return false
	}
	return session.PIDAlive(rec.CagePID, rec.CagePIDStart)
}

func waitNoProcesses(name string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if len(screenProcesses(name)) == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}
