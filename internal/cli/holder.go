package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hexadecimil/hyprcage/internal/registry"
	"github.com/hexadecimil/hyprcage/internal/screen"
)

// runHolder is cage's child process (cahier §4.1): it publishes cage's
// sockets in <registry>/<screen>.inner and then blocks. cage exits when its
// child exits, so this process is the screen's point of voluntary death.
func runHolder(e *Env) int {
	if len(e.Args) != 1 {
		return e.errorf("usage: hyprcage _holder <screen>")
	}
	name := e.Args[0]
	if err := screen.ValidateName(name); err != nil {
		return e.errorf("%v", err)
	}
	disp := os.Getenv("WAYLAND_DISPLAY")
	if disp == "" {
		return e.errorf("_holder: WAYLAND_DISPLAY is empty; it must run as cage's child")
	}
	if _, err := registry.EnsureDir(); err != nil {
		return e.errorf("%v", err)
	}
	data := fmt.Sprintf("WAYLAND_DISPLAY=%s\nDISPLAY=%s\n", disp, os.Getenv("DISPLAY"))
	if err := registry.WriteAtomic(registry.InnerPath(name), []byte(data), 0o600); err != nil {
		return e.errorf("%v", err)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	parent := os.Getppid()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-sig:
			return ExitOK
		case <-tick.C:
			if os.Getppid() != parent {
				return ExitOK // cage is gone: nothing left to hold
			}
		}
	}
}
