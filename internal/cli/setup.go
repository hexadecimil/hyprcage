package cli

import (
	"fmt"
	"strings"

	"github.com/hexadecimil/hyprcage/internal/setup"
)

// runSetup installs cage and wl-mirror with the human's authorisation
// (sudo in a terminal, a polkit dialog otherwise).
func runSetup(e *Env) int {
	fs := e.flags("setup")
	asJSON := fs.Bool("json", false, "JSON output")
	if err := e.parse(fs); err != nil {
		return ExitUsage
	}
	rep, err := setup.Run()
	if *asJSON {
		if err != nil {
			rep.Manual = err.Error()
		}
		return e.printJSON(rep)
	}
	switch {
	case err != nil:
		return e.fail(err)
	case len(rep.Missing) == 0:
		fmt.Fprintln(e.Stdout, "nothing to install: cage and wl-mirror are present")
	default:
		fmt.Fprintf(e.Stdout, "installed %s via %s\n", strings.Join(rep.Installed, ", "), rep.Method)
	}
	return ExitOK
}
