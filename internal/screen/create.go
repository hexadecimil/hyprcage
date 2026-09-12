package screen

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/hexadecimil/hyprcage/internal/hypr"
	"github.com/hexadecimil/hyprcage/internal/lock"
	"github.com/hexadecimil/hyprcage/internal/notify"
	"github.com/hexadecimil/hyprcage/internal/registry"
	"github.com/hexadecimil/hyprcage/internal/shellq"
	"github.com/hexadecimil/hyprcage/internal/sysd"
	"github.com/hexadecimil/hyprcage/internal/wl"
)

// CreateOptions are the parameters of screen_create (cahier F1).
type CreateOptions struct {
	Name   string
	Width  int
	Height int
	Mirror bool
	Owner  registry.Owner
}

// Create implements cahier §4.2. Step 7's check of cage's four Wayland
// globals waits for the wl package (spike S7); until then the cage window
// showing up fullscreen in Hyprland is the readiness signal.
func Create(c *Ctx, opts CreateOptions) (*registry.Screen, error) {
	cfg := c.Cfg
	if opts.Name == "" {
		opts.Name = NewName(cfg.OutputPrefix)
	}
	if err := ValidateName(opts.Name); err != nil {
		return nil, err
	}
	if opts.Width <= 0 || opts.Height <= 0 {
		opts.Width, opts.Height = cfg.DefaultWidth, cfg.DefaultHeight
	}
	if opts.Width < 320 || opts.Height < 240 || opts.Width > cfg.MaxWidth || opts.Height > cfg.MaxHeight {
		return nil, errf(CodeLimit, "screen.max_width / screen.max_height in config.toml", "size %dx%d is outside 320x240 .. %dx%d", opts.Width, opts.Height, cfg.MaxWidth, cfg.MaxHeight)
	}
	if _, err := exec.LookPath("cage"); err != nil {
		return nil, errf(CodeCage, "run `hyprcage setup` (or the setup tool): it installs cage after the human enters their password", "cage not found in PATH")
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}

	release, err := lock.Acquire(30 * time.Second)
	if err != nil {
		return nil, err
	}
	defer release()

	// Step 1: reap orphans first (M4).
	_, _ = GC(c, GCOptions{})
	if _, err := registry.Load(opts.Name); err == nil {
		return nil, errf(CodeInvalidName, "pick another name", "screen %s already exists", opts.Name)
	}
	if cfg.MaxPerSession > 0 {
		recs, _ := registry.List()
		n := 0
		for _, r := range recs {
			if sameOwner(r.Owner, opts.Owner) {
				n++
			}
		}
		if n >= cfg.MaxPerSession {
			return nil, errf(CodeLimit, "destroy a screen you no longer need, or raise screen.max_per_session in config.toml", "this session already holds %d screen(s), its limit", n)
		}
	}

	// Step 2: workspaces.
	wss, err := c.Hypr.Workspaces()
	if err != nil {
		return nil, errf(CodeHyprland, "", "%v", err)
	}
	used := map[int]bool{}
	for _, w := range wss {
		used[w.ID] = true
	}
	wsApp := freeWorkspace(used, cfg.WorkspaceMin, cfg.WorkspaceMax)
	if wsApp == 0 {
		return nil, fmt.Errorf("no free workspace in [%d, %d]", cfg.WorkspaceMin, cfg.WorkspaceMax)
	}
	wsMirror := 0
	if opts.Mirror && cfg.MirrorEnabled {
		if _, err := exec.LookPath("wl-mirror"); err == nil {
			wsMirror = freeWorkspace(used, cfg.MirrorMin, cfg.MirrorMax)
		}
	}

	// Step 3: declare the output and its workspace before creating it.
	mons, err := c.Hypr.Monitors()
	if err != nil {
		return nil, errf(CodeHyprland, "", "%v", err)
	}
	posX, posY := farPosition(mons, opts.Width)
	if err := c.Driver.DeclareMonitor(opts.Name, opts.Width, opts.Height, cfg.RefreshHz, posX, posY, 1); err != nil {
		return nil, err
	}
	if err := c.Driver.WorkspaceRule(wsApp, opts.Name); err != nil {
		return nil, err
	}
	if wsMirror > 0 {
		// A gap-free rule on the mirror workspace lets the mirror window
		// tile to the full monitor; it is not pinned to a monitor (the
		// human moves it with SUPER+n), so no monitor field.
		_ = c.Driver.WorkspaceGapless(wsMirror)
	}

	// Step 4: record and safety timer, before anything exists (N2).
	rec := &registry.Screen{
		Name: opts.Name, CreatedAt: time.Now(), State: "starting",
		Width: opts.Width, Height: opts.Height, PosX: posX, PosY: posY,
		WorkspaceApp: wsApp, WorkspaceMirror: wsMirror,
		Slice: sysd.SliceName(opts.Name), Owner: opts.Owner,
	}
	if err := registry.Save(rec); err != nil {
		return nil, err
	}
	useSystemd := sysd.Available()
	if useSystemd {
		if err := sysd.ArmTimer(sysd.GCTimerName(opts.Name), cfg.SafetyTimer, []string{exe, "gc"}); err != nil {
			_ = registry.Delete(opts.Name)
			return nil, fmt.Errorf("arming the safety timer: %w", err)
		}
	}
	fail := func(err error) (*registry.Screen, error) {
		_ = Destroy(c, rec)
		return nil, err
	}

	// Step 5: create the output and check its geometry.
	if err := c.Hypr.CreateHeadless(opts.Name); err != nil {
		return fail(err)
	}
	mon, err := waitMonitor(c.Hypr, opts.Name, 2*time.Second)
	if err != nil {
		return fail(err)
	}
	if mon.Width != opts.Width || mon.Height != opts.Height || mon.Scale != 1 {
		_ = c.Driver.DeclareMonitor(opts.Name, opts.Width, opts.Height, cfg.RefreshHz, posX, posY, 1)
		time.Sleep(200 * time.Millisecond)
		if mon, err = waitMonitor(c.Hypr, opts.Name, time.Second); err != nil {
			return fail(err)
		}
		if mon.Width != opts.Width || mon.Height != opts.Height || mon.Scale != 1 {
			return fail(fmt.Errorf("output %s is %dx%d scale %v, wanted %dx%d scale 1 (a catch-all monitor rule may override it)",
				opts.Name, mon.Width, mon.Height, mon.Scale, opts.Width, opts.Height))
		}
	}
	// A desktop shell may put its bar on every output, this one included
	// (Omarchy does): the bar's exclusive zone shrinks the workspace and the
	// tiled cage window with it. The output is enlarged by that reserved
	// area so that the cage window keeps exactly opts.Width x opts.Height;
	// the agent's screen is the cage window, captured inside cage, never
	// the Hyprland output. A bar that attaches a moment later is handled
	// below, when the cage window turns out too small.
	waitForBar(c, opts.Name, cfg.OutputPrefix)
	if changed, err := fitReserved(c, rec); err != nil {
		return fail(err)
	} else if changed {
		// The bar re-creates its layer after the mode change; cage must not
		// start while the output is mid-transient (a client that maps then
		// never receives input from cage).
		waitReservedSettled(c, rec, 3*time.Second)
	}

	// Step 6 and 7: cage, launched by Hyprland so that the exec rules apply
	// by pid; then its inner socket and its window. The renderer is tried in
	// order (see renderer.go): a candidate that maps no window within a few
	// seconds is killed and the next one is tried.
	// No fullscreen exec rule: Hyprland 0.56 ignores it; the workspace rule
	// (no gaps, no border) makes the tiled window cover the output exactly.
	rules := hypr.ExecRules{Workspace: fmt.Sprintf("%d silent", wsApp), NoInitialFocus: true, NoAnim: true}
	candidates := rendererCandidates(cfg.Renderer)
	var win *hypr.Client
	for i, renderer := range candidates {
		_ = os.Remove(registry.InnerPath(opts.Name))
		cageCmd := []string{"env", "HYPRCAGE_SCREEN=" + opts.Name}
		if renderer != "" {
			cageCmd = append(cageCmd, "WLR_RENDERER="+renderer)
		}
		cageCmd = append(cageCmd, "cage", "-d", "--", exe, "_holder", opts.Name)
		argv := cageCmd
		if useSystemd {
			argv = sysd.ScopeArgs(opts.Name, "cage", cageCmd)
		}
		if err := c.Driver.Exec(shellq.Join(argv), rules); err != nil {
			return fail(err)
		}
		inner, err := waitInner(opts.Name, 5*time.Second)
		if err != nil {
			return fail(err)
		}
		rec.InnerDisplay, rec.InnerX11 = inner["WAYLAND_DISPLAY"], inner["DISPLAY"]
		var tooSmall bool
		win, tooSmall, err = waitCageWindow(c.Hypr, wsApp, opts.Width, opts.Height, 4*time.Second)
		if tooSmall {
			if changed, ferr := fitReserved(c, rec); ferr == nil && changed {
				win, _, err = waitCageWindow(c.Hypr, wsApp, opts.Width, opts.Height, 4*time.Second)
			}
		}
		if err == nil {
			// The window maps under GLES even on a virtualized GPU (virgl),
			// but screencopy then hangs there. Probe one capture: a renderer
			// that cannot be captured is useless, so fall back like a window
			// that never appeared.
			if perr := probeCapture(rec.InnerDisplay); perr != nil {
				err = perr
			}
		}
		if err == nil {
			if renderer != "" {
				rememberRenderer(renderer)
			}
			break
		}
		if i == len(candidates)-1 {
			return fail(err)
		}
		rememberRenderer(rendererPixman) // next candidate; skip GLES next time
		killCage(opts.Name, useSystemd)
	}
	rec.CagePID = win.PID
	if rec.Reserved != [4]int{} {
		// Hand the screen over only once the cage window has held its size
		// for a moment: cage's own output follows every resize.
		waitCageWindowHeld(c.Hypr, wsApp, opts.Width, opts.Height, 800*time.Millisecond, 4*time.Second)
	}

	// Step 8: mirror for the human, optional.
	if wsMirror > 0 {
		// The gap-free workspace rule already makes the tiled mirror cover
		// its workspace, so wl-mirror's own -F (an xdg fullscreen request)
		// is not used: verified on 0.56.2, -F makes the window ignore the
		// "workspace N silent" rule and land fullscreen on the human's
		// current workspace instead. Under software rendering its dmabuf
		// backends cannot allocate, so screencopy over shm is forced.
		mirrorCmd := []string{"wl-mirror"}
		if KnownRenderer(cfg.Renderer) == rendererPixman {
			mirrorCmd = append(mirrorCmd, "-b", "screencopy-shm")
		}
		mirrorCmd = append(mirrorCmd, opts.Name)
		margv := mirrorCmd
		if useSystemd {
			margv = sysd.ScopeArgs(opts.Name, "mirror", mirrorCmd)
		}
		mrules := hypr.ExecRules{Workspace: fmt.Sprintf("%d silent", wsMirror), NoInitialFocus: true, NoAnim: true}
		if err := c.Driver.Exec(shellq.Join(margv), mrules); err != nil {
			rec.WorkspaceMirror = 0
		} else {
			waitMirrorWindow(c.Hypr, opts.Name, wsMirror, 4*time.Second) // best effort
		}
	}

	// Step 9: the watcher re-asserts the geometry after `hyprctl reload`
	// (N11); it lives in the screen's slice and dies with it.
	watchCmd := []string{exe, "_watch", opts.Name}
	if useSystemd {
		watchCmd = sysd.ScopeArgs(opts.Name, "watch", watchCmd)
	}
	startDetached(watchCmd)
	rec.State = "ready"
	if err := registry.Save(rec); err != nil {
		return fail(err)
	}
	if cfg.Notify {
		body := opts.Name
		if rec.WorkspaceMirror > 0 {
			body = fmt.Sprintf("%s, mirror on workspace %d", opts.Name, rec.WorkspaceMirror)
		}
		notify.Send("Agent screen opened", body)
	}
	return rec, nil
}

// Reassert re-applies the output declaration and the workspace rule of a
// screen, after a Hyprland config reload wiped the runtime ones. The output
// is declared with the reserved area remembered in the record, not with a
// live reading: a bar re-creates its layer around every output change, and
// a reading taken in that instant would say there is no bar.
func Reassert(c *Ctx, rec *registry.Screen) error {
	w, h := fittedSize(rec.Width, rec.Height, rec.Reserved)
	if err := c.Driver.DeclareMonitor(rec.Name, w, h, c.Cfg.RefreshHz, rec.PosX, rec.PosY, 1); err != nil {
		return err
	}
	return c.Driver.WorkspaceRule(rec.WorkspaceApp, rec.Name)
}

// Refit re-declares the output of a screen if its reserved area changed (a
// bar appeared on it, or went away for good), so that the cage window keeps
// the screen's size, and records the new reserved area. It reports whether
// the output changed. Callers must make sure the reserved area is settled:
// see the watcher.
func Refit(c *Ctx, rec *registry.Screen) (bool, error) {
	changed, err := fitReserved(c, rec)
	if err == nil && changed {
		err = registry.Save(rec)
	}
	return changed, err
}

// ReservedArea reads the current reserved area of the screen's output.
func ReservedArea(c *Ctx, rec *registry.Screen) ([4]int, bool) {
	mons, err := c.Hypr.Monitors()
	if err != nil {
		return [4]int{}, false
	}
	for _, m := range mons {
		if m.Name == rec.Name {
			return m.Reserved, true
		}
	}
	return [4]int{}, false
}

// waitForBar gives a desktop bar that sits on every output the time to land
// on the new one, so that the output is fitted before cage maps rather than
// a few seconds after. It only waits when a real monitor carries a reserved
// area (the system has such a bar), and at most barGrace: a bar confined
// to one monitor costs that much once per screen, a system without a bar
// costs nothing.
func waitForBar(c *Ctx, name, prefix string) {
	mons, err := c.Hypr.Monitors()
	if err != nil {
		return
	}
	hasBar := false
	for _, m := range mons {
		if !strings.HasPrefix(m.Name, prefix) && m.Reserved != [4]int{} {
			hasBar = true
		}
	}
	if !hasBar {
		return
	}
	deadline := time.Now().Add(barGrace)
	for time.Now().Before(deadline) {
		if m, err := waitMonitor(c.Hypr, name, time.Second); err == nil && m.Reserved != [4]int{} {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

const barGrace = 3 * time.Second

// waitReservedSettled returns once the output's reserved area has been the
// recorded one, without change, for half a second (the bar remapped after
// a mode change), or after timeout.
func waitReservedSettled(c *Ctx, rec *registry.Screen, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	var since time.Time
	for time.Now().Before(deadline) {
		if r, ok := ReservedArea(c, rec); ok && r == rec.Reserved && r != [4]int{} {
			if since.IsZero() {
				since = time.Now()
			} else if time.Since(since) >= 500*time.Millisecond {
				return
			}
		} else {
			since = time.Time{}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// waitCageWindowHeld returns once the cage window has kept width x height
// (or fullscreen) for hold, or after timeout.
func waitCageWindowHeld(h *hypr.Instance, ws, width, height int, hold, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	var since time.Time
	for time.Now().Before(deadline) {
		ok := false
		if cls, err := h.Clients(); err == nil {
			for i := range cls {
				if cls[i].Workspace.ID == ws && (cls[i].Fullscreen != 0 || (cls[i].Size[0] == width && cls[i].Size[1] == height)) {
					ok = true
				}
			}
		}
		if !ok {
			since = time.Time{}
		} else if since.IsZero() {
			since = time.Now()
		} else if time.Since(since) >= hold {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// fittedSize is the output size that leaves width x height to the tiled
// window once a reserved area (left, top, right, bottom) is taken out.
func fittedSize(width, height int, reserved [4]int) (int, int) {
	return width + reserved[0] + reserved[2], height + reserved[1] + reserved[3]
}

// fitReserved re-declares the output enlarged by its current reserved area
// and waits for the new mode; it records the reserved area in rec and
// reports whether the output changed.
func fitReserved(c *Ctx, rec *registry.Screen) (bool, error) {
	mon, err := waitMonitor(c.Hypr, rec.Name, time.Second)
	if err != nil {
		return false, err
	}
	rec.Reserved = mon.Reserved
	w, h := fittedSize(rec.Width, rec.Height, mon.Reserved)
	if mon.Width == w && mon.Height == h {
		return false, nil
	}
	if err := c.Driver.DeclareMonitor(rec.Name, w, h, c.Cfg.RefreshHz, rec.PosX, rec.PosY, 1); err != nil {
		return false, err
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		m, err := waitMonitor(c.Hypr, rec.Name, time.Second)
		if err != nil {
			return false, err
		}
		if m.Width == w && m.Height == h {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, fmt.Errorf("output %s is %dx%d, wanted %dx%d to absorb a reserved area of %v", rec.Name, m.Width, m.Height, w, h, mon.Reserved)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// startDetached runs argv in its own session, stdio to /dev/null, reaped
// in the background.
func startDetached(argv []string) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return
	}
	go func() { _ = cmd.Wait() }()
}

// probeCapture checks that screencopy actually completes on a screen, with a
// short deadline: it hangs under virgl's GLES path, so a timeout here means
// the renderer must change. The abandoned connection is closed on return.
func probeCapture(display string) error {
	cl, err := wl.Connect(display)
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() {
		_, e := cl.Capture(false)
		done <- e
	}()
	select {
	case e := <-done:
		cl.Close()
		return e
	case <-time.After(6 * time.Second):
		cl.Close()
		return errf(CodeCage, "the GPU renderer cannot be captured; forcing pixman", "screencopy did not complete")
	}
}

// killCage stops a cage that never mapped a window: its holder is killed
// (cage exits when its child exits), the scope stopped when systemd is used.
func killCage(name string, useSystemd bool) {
	if useSystemd {
		_ = sysd.StopUnit(sysd.UnitName(name, "cage") + ".scope")
	}
	_ = exec.Command("pkill", "-f", "_holder "+name+"$").Run()
	time.Sleep(300 * time.Millisecond)
}

// waitMirrorWindow waits, best effort, for wl-mirror's window to map so that
// screen_create returns with the mirror already there for the human.
func waitMirrorWindow(h *hypr.Instance, name string, ws int, timeout time.Duration) {
	title := "Wayland Output Mirror for " + name
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		cls, err := h.Clients()
		if err == nil {
			for i := range cls {
				if cls[i].Workspace.ID == ws && cls[i].Title == title {
					return
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func freeWorkspace(used map[int]bool, min, max int) int {
	for i := min; i <= max; i++ {
		if !used[i] {
			return i
		}
	}
	return 0
}

// farGap is the distance kept between the agent's output and everything else.
const farGap = 10000

// farPosition places the output far to the LEFT of every monitor, with its
// right edge at or below 0. Verified on Hyprland 0.56.2: a monitor placed to
// the right (or with a right edge above 0) makes every `auto`-positioned
// real monitor jump next to it, which is the opposite of a barrier; a
// negative position leaves `auto` monitors where they are. The gap keeps the
// output non-adjacent, and Hyprland clamps the cursor to the nearest monitor
// box, so the human's pointer can never reach it (cahier §2.2.1).
func farPosition(mons []hypr.Monitor, width int) (int, int) {
	minX := 0
	for _, m := range mons {
		if m.X < minX {
			minX = m.X
		}
	}
	return minX - farGap - width, 0
}

func waitMonitor(h *hypr.Instance, name string, timeout time.Duration) (*hypr.Monitor, error) {
	deadline := time.Now().Add(timeout)
	for {
		mons, err := h.Monitors()
		if err == nil {
			for i := range mons {
				if mons[i].Name == name {
					return &mons[i], nil
				}
			}
		}
		if time.Now().After(deadline) {
			return nil, errf(CodeTimeout, "", "output %s did not appear", name)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitInner(name string, timeout time.Duration) (map[string]string, error) {
	deadline := time.Now().Add(timeout)
	for {
		m, err := registry.ReadInner(name)
		if err == nil && m["WAYLAND_DISPLAY"] != "" {
			return m, nil
		}
		if time.Now().After(deadline) {
			return nil, errf(CodeCage, "journalctl --user -u "+sysd.UnitName(name, "cage"), "cage did not publish its socket within %s", timeout)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// waitCageWindow waits for cage's window on the workspace, covering the
// output exactly: fullscreen, or tiled at the output's size (workspace rule).
// waitCageWindow waits for the cage window on the workspace to be
// fullscreen or exactly width x height. A window that stays at another size
// is reported early (tooSmall) so that the caller can absorb a reserved
// area and try again.
func waitCageWindow(h *hypr.Instance, ws, width, height int, timeout time.Duration) (win *hypr.Client, tooSmall bool, err error) {
	deadline := time.Now().Add(timeout)
	var seen *hypr.Client
	var mismatchSince time.Time
	for {
		cls, cerr := h.Clients()
		if cerr == nil {
			for i := range cls {
				if cls[i].Workspace.ID != ws {
					continue
				}
				seen = &cls[i]
				if cls[i].Fullscreen != 0 || (cls[i].Size[0] == width && cls[i].Size[1] == height) {
					return &cls[i], false, nil
				}
				if mismatchSince.IsZero() {
					mismatchSince = time.Now()
				}
			}
		}
		stuck := !mismatchSince.IsZero() && time.Since(mismatchSince) > 700*time.Millisecond
		if time.Now().After(deadline) || stuck {
			if seen != nil {
				return nil, true, errf(CodeCage, "a bar or reserved area may shrink the workspace", "cage window on workspace %d is %dx%d, wanted %dx%d", ws, seen.Size[0], seen.Size[1], width, height)
			}
			return nil, false, errf(CodeCage, "the exec rules may not have applied (spike S6)", "cage window did not appear on workspace %d", ws)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
