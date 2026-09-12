package cli

import (
	"fmt"
	"strings"

	"github.com/hexadecimil/hyprcage/internal/config"
	"github.com/hexadecimil/hyprcage/internal/screen"
	"github.com/hexadecimil/hyprcage/internal/session"
)

type multiFlag []string

func (m *multiFlag) String() string     { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error { *m = append(*m, v); return nil }

func runLaunch(e *Env) int {
	fs := e.flags("launch")
	cwd := fs.String("cwd", "", "working directory of the application")
	var envs multiFlag
	fs.Var(&envs, "env", "KEY=VALUE for the application (repeatable)")
	if err := e.parse(fs); err != nil {
		return ExitUsage
	}
	if fs.NArg() < 2 {
		return e.errorf("usage: hyprcage launch [--cwd D] [--env K=V]... <screen> -- <command...>")
	}
	cfg, err := config.Load()
	if err != nil {
		return e.fail(err)
	}
	c, err := screen.Connect(cfg)
	if err != nil {
		return e.fail(err)
	}
	rec, err := screen.Resolve(c, fs.Arg(0), session.Current())
	if err != nil {
		return e.fail(err)
	}
	extra := map[string]string{}
	for _, kv := range envs {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return e.errorf("--env expects KEY=VALUE, got %q", kv)
		}
		extra[k] = v
	}
	pid, logPath, err := screen.Launch(c, rec, fs.Args()[1:], *cwd, extra)
	if err != nil {
		return e.fail(err)
	}
	fmt.Fprintf(e.Stdout, "pid=%d\nlog=%s\n", pid, logPath)
	return ExitOK
}
