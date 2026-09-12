package cli

import (
	"fmt"

	"github.com/hexadecimil/hyprcage/internal/config"
	"github.com/hexadecimil/hyprcage/internal/screen"
	"github.com/hexadecimil/hyprcage/internal/session"
)

func runCreate(e *Env) int {
	fs := e.flags("create")
	size := fs.String("size", "", "WxH (default 1280x800)")
	name := fs.String("name", "", "screen name (default hc-<random>)")
	noMirror := fs.Bool("no-mirror", false, "do not open a mirror window for the human")
	asJSON := fs.Bool("json", false, "print the record as JSON")
	if err := e.parse(fs); err != nil {
		return ExitUsage
	}
	w, h, err := parseSize(*size)
	if err != nil {
		return e.errorf("%v", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return e.fail(err)
	}
	c, err := screen.Connect(cfg)
	if err != nil {
		return e.fail(err)
	}
	rec, err := screen.Create(c, screen.CreateOptions{
		Name: *name, Width: w, Height: h, Mirror: !*noMirror,
		Owner: session.Current().Owner(),
	})
	if err != nil {
		return e.fail(err)
	}
	if *asJSON {
		return e.printJSON(rec)
	}
	fmt.Fprintf(e.Stdout, "name=%s\nsize=%dx%d\nworkspace_app=%d\nworkspace_mirror=%d\ninner_display=%s\n",
		rec.Name, rec.Width, rec.Height, rec.WorkspaceApp, rec.WorkspaceMirror, rec.InnerDisplay)
	return ExitOK
}

func parseSize(s string) (int, int, error) {
	if s == "" {
		return 0, 0, nil
	}
	var w, h int
	if _, err := fmt.Sscanf(s, "%dx%d", &w, &h); err != nil || w <= 0 || h <= 0 {
		return 0, 0, fmt.Errorf("invalid --size %q, expected WxH", s)
	}
	return w, h, nil
}
