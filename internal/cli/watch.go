package cli

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hexadecimil/hyprcage/internal/config"
	"github.com/hexadecimil/hyprcage/internal/hypr"
	"github.com/hexadecimil/hyprcage/internal/registry"
	"github.com/hexadecimil/hyprcage/internal/screen"
)

// runWatch keeps one screen's geometry alive (cahier N11): it listens to
// Hyprland's event socket and re-applies the output declaration and the
// workspace rule after a config reload, which otherwise hands the output
// to the catch-all monitor rule (preferred mode, `auto` position next to
// the human's monitors). It also refits the output when a layer surface
// opens or closes: a desktop bar that lands on the agent output reserves
// part of it and would shrink the cage window. A bar re-creates its layer
// around every output change, so layer events are debounced and the
// reserved area has to hold still before the output follows it; otherwise
// the watcher and the bar chase each other forever. It runs in the
// screen's slice and exits when the screen's record disappears.
func runWatch(e *Env) int {
	if len(e.Args) != 1 {
		return e.errorf("usage: hyprcage _watch <screen>")
	}
	name := e.Args[0]
	if err := screen.ValidateName(name); err != nil {
		return e.errorf("%v", err)
	}
	c, err := screen.Connect(config.Fallback())
	if err != nil {
		return e.fail(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	go func() { <-sig; cancel() }()
	go func() { // exit with the record
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if _, err := registry.Load(name); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	r := &refitter{c: c, name: name}
	r.schedule() // a bar may have landed between the window check and now
	for ctx.Err() == nil {
		err := c.Hypr.Subscribe(ctx, func(ev hypr.Event) {
			switch ev.Name {
			case "configreloaded":
				reassert(c, name)
				r.schedule()
			case "monitoradded", "monitoraddedv2":
				if strings.Contains(ev.Data, name) {
					reassert(c, name)
					r.schedule()
				}
			case "openlayer", "closelayer":
				r.schedule()
			}
		})
		if ctx.Err() != nil {
			break
		}
		if err != nil { // Hyprland restarted or the socket dropped: retry
			time.Sleep(time.Second)
		}
	}
	return ExitOK
}

// refitter follows the reserved area of the output with a settling delay:
// a check runs settleDelay after the last layer event, and only refits when
// two readings settleGap apart agree.
type refitter struct {
	c     *screen.Ctx
	name  string
	mu    sync.Mutex
	timer *time.Timer
}

const (
	settleDelay = 1500 * time.Millisecond
	settleGap   = 300 * time.Millisecond
)

func (r *refitter) schedule() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.timer != nil {
		r.timer.Stop()
	}
	r.timer = time.AfterFunc(settleDelay, r.check)
}

func (r *refitter) check() {
	rec, err := registry.Load(r.name)
	if err != nil {
		return
	}
	first, ok := screen.ReservedArea(r.c, rec)
	if !ok {
		return
	}
	time.Sleep(settleGap)
	second, ok := screen.ReservedArea(r.c, rec)
	if !ok {
		return
	}
	if first != second { // still moving: look again later
		r.schedule()
		return
	}
	_, _ = screen.Refit(r.c, rec)
}

func reassert(c *screen.Ctx, name string) {
	rec, err := registry.Load(name)
	if err != nil {
		return
	}
	time.Sleep(200 * time.Millisecond) // let the reload finish applying
	_ = screen.Reassert(c, rec)
}
