package screen

import (
	"fmt"
	"os"
	"path/filepath"
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

// captureHuman reads the state of the human's monitors. known is the set of
// output names hyprcage owns, read once by the caller: everything that is
// neither in it nor under the prefix belongs to the human.
func captureHuman(h *hypr.Instance, prefix string, known map[string]bool) *humanState {
	st := &humanState{active: map[string]int{}}
	st.cursor, _ = h.CursorPos()
	mons, err := h.Monitors()
	if err != nil {
		return st
	}
	for _, m := range mons {
		if isAgentOutput(m.Name, prefix, known) {
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
// classic and the Lua mode. The cursor alone is put back only when
// cursorAlone is set: later passes leave a cursor the human may be moving.
func restoreHuman(h *hypr.Instance, d hypr.ConfigDriver, before *humanState, prefix string, known map[string]bool, cursorAlone bool) bool {
	after := captureHuman(h, prefix, known)
	var cmds []string
	var moved []string
	for name, ws := range before.active {
		if after.active[name] != ws {
			cmds = append(cmds, d.FocusMonitorCmd(name), d.WorkspaceCmd(ws))
			moved = append(moved, fmt.Sprintf("%s: workspace %d -> %d", name, ws, after.active[name]))
		}
	}
	if after.cursor != before.cursor {
		moved = append(moved, fmt.Sprintf("cursor: %d,%d -> %d,%d", before.cursor.X, before.cursor.Y, after.cursor.X, after.cursor.Y))
	}
	if after.focused != before.focused {
		moved = append(moved, fmt.Sprintf("focus: %s -> %s", before.focused, after.focused))
	}
	if len(moved) > 0 {
		traceRestore(strings.Join(moved, "; "), len(cmds) > 0 || cursorAlone)
	}
	if len(cmds) > 0 && before.focused != "" {
		cmds = append(cmds, d.FocusMonitorCmd(before.focused))
	}
	if len(cmds) > 0 || (cursorAlone && after.cursor != before.cursor) {
		cmds = append(cmds, d.MoveCursorCmd(before.cursor.X, before.cursor.Y))
	}
	if len(cmds) > 0 {
		_, _ = h.Batch(cmds...)
		return true
	}
	return false
}

// removeOutputRestoring removes the output and keeps the human's state in
// place while Hyprland reacts. The workspaces of a removed output are
// migrated a moment after `output remove` returns, and a desktop's own
// monitor scripts may react after that, so one comparison right after the
// removal can see nothing to restore: the state is watched for a while and
// put back every time it moves.
func removeOutputRestoring(c *Ctx, name string) error {
	known := knownScreens()
	before := captureHuman(c.Hypr, c.Cfg.OutputPrefix, known)
	err := c.Hypr.RemoveOutput(name)
	waitOutputGone(c.Hypr, name, time.Second)
	restoreHuman(c.Hypr, c.Driver, before, c.Cfg.OutputPrefix, known, true)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(150 * time.Millisecond)
		restoreHuman(c.Hypr, c.Driver, before, c.Cfg.OutputPrefix, known, false)
	}
	return err
}

// traceRestore appends what `output remove` moved, and whether it was put
// back, to ~/.local/state/hyprcage/log/restore.log: the record to read when
// a human reports that a workspace or the cursor changed under them.
func traceRestore(what string, restored bool) {
	f, err := os.OpenFile(filepath.Join(LogDir(), "restore.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	verb := "seen, left alone"
	if restored {
		verb = "restored"
	}
	fmt.Fprintf(f, "%s %s (%s)\n", time.Now().Format(time.RFC3339Nano), what, verb)
}

func waitOutputGone(h *hypr.Instance, name string, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for hasMonitor(h, name) && time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
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
		removeErr = removeOutputRestoring(c, rec.Name)
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
