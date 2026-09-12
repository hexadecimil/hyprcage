package screen

import (
	"strings"
	"syscall"
	"time"

	"github.com/hexadecimil/hyprcage/internal/hypr"
	"github.com/hexadecimil/hyprcage/internal/lock"
	"github.com/hexadecimil/hyprcage/internal/notify"
	"github.com/hexadecimil/hyprcage/internal/registry"
	"github.com/hexadecimil/hyprcage/internal/sysd"
)

// humanState is what `output remove` may disturb (cahier §4.3).
type humanState struct {
	cursor  hypr.CursorPos
	active  map[string]int // real monitor → active workspace id
	focused string
}

func captureHuman(h *hypr.Instance, prefix string) *humanState {
	st := &humanState{active: map[string]int{}}
	st.cursor, _ = h.CursorPos()
	mons, err := h.Monitors()
	if err != nil {
		return st
	}
	for _, m := range mons {
		if strings.HasPrefix(m.Name, prefix) {
			continue
		}
		st.active[m.Name] = m.ActiveWorkspace.ID
		if m.Focused {
			st.focused = m.Name
		}
	}
	return st
}

// restoreHuman puts back the state captured before `output remove`: the only
// documented exception to N3.1, bounded to restoring what Hyprland changed
// (a warped cursor, a migrated workspace made active on a real monitor).
// The dispatchers come from the driver: their spelling differs between the
// classic and the Lua mode.
func restoreHuman(h *hypr.Instance, d hypr.ConfigDriver, before *humanState, prefix string) {
	after := captureHuman(h, prefix)
	var cmds []string
	for name, ws := range before.active {
		if after.active[name] != ws {
			cmds = append(cmds, d.FocusMonitorCmd(name), d.WorkspaceCmd(ws))
		}
	}
	if len(cmds) > 0 && before.focused != "" {
		cmds = append(cmds, d.FocusMonitorCmd(before.focused))
	}
	if len(cmds) > 0 || after.cursor != before.cursor {
		cmds = append(cmds, d.MoveCursorCmd(before.cursor.X, before.cursor.Y))
	}
	if len(cmds) > 0 {
		_, _ = h.Batch(cmds...)
	}
}

// Destroy implements cahier §4.3. Ownership is checked by the caller.
func Destroy(c *Ctx, rec *registry.Screen) error {
	release, err := lock.Acquire(30 * time.Second)
	if err != nil {
		return err
	}
	defer release()

	useSystemd := sysd.Available()
	if useSystemd {
		_ = sysd.StopUnit(rec.Slice) // SIGTERM the whole cgroup, SIGKILL after 3 s
	} else {
		killScreenPIDs(c.Hypr, rec, syscall.SIGTERM)
	}
	if !waitNoClients(c.Hypr, rec, 3*time.Second) {
		killScreenPIDs(c.Hypr, rec, syscall.SIGKILL)
		waitNoClients(c.Hypr, rec, time.Second)
	}

	var removeErr error
	if hasMonitor(c.Hypr, rec.Name) {
		before := captureHuman(c.Hypr, c.Cfg.OutputPrefix)
		removeErr = c.Hypr.RemoveOutput(rec.Name)
		restoreHuman(c.Hypr, c.Driver, before, c.Cfg.OutputPrefix)
	}

	if useSystemd {
		sysd.DisarmTimer(sysd.GCTimerName(rec.Name))
		sysd.ResetFailed(rec.Slice)
	}
	if err := registry.Delete(rec.Name); err != nil {
		return err
	}
	if c.Cfg.Notify {
		notify.Send("Agent screen closed", rec.Name)
	}
	return removeErr
}

func hasMonitor(h *hypr.Instance, name string) bool {
	mons, err := h.Monitors()
	if err != nil {
		return false
	}
	for _, m := range mons {
		if m.Name == name {
			return true
		}
	}
	return false
}

// screenPIDs lists the windows of the screen: everything on its app
// workspace, and only the mirror window on the (shared) mirror workspace.
func screenPIDs(h *hypr.Instance, rec *registry.Screen) []int {
	cls, err := h.Clients()
	if err != nil {
		return nil
	}
	var pids []int
	for _, cl := range cls {
		onApp := cl.Workspace.ID == rec.WorkspaceApp
		isMirror := rec.WorkspaceMirror > 0 && cl.Workspace.ID == rec.WorkspaceMirror &&
			cl.Title == "Wayland Output Mirror for "+rec.Name
		if onApp || isMirror {
			pids = append(pids, cl.PID)
		}
	}
	return pids
}

func killScreenPIDs(h *hypr.Instance, rec *registry.Screen, sig syscall.Signal) {
	for _, pid := range screenPIDs(h, rec) {
		if pid > 0 {
			_ = syscall.Kill(pid, sig)
		}
	}
}

func waitNoClients(h *hypr.Instance, rec *registry.Screen, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if len(screenPIDs(h, rec)) == 0 {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(100 * time.Millisecond)
	}
}
