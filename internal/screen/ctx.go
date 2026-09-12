package screen

import (
	"github.com/hexadecimil/hyprcage/internal/config"
	"github.com/hexadecimil/hyprcage/internal/hypr"
)

// Ctx bundles what every operation needs: the configuration, the Hyprland
// instance and its configuration driver.
type Ctx struct {
	Cfg    config.Config
	Hypr   *hypr.Instance
	Driver hypr.ConfigDriver
}

// Connect discovers Hyprland and picks the configuration driver.
func Connect(cfg config.Config) (*Ctx, error) {
	inst, err := hypr.Discover()
	if err != nil {
		return nil, errf(CodeHyprland, "is Hyprland running? HYPRLAND_INSTANCE_SIGNATURE or XDG_RUNTIME_DIR may be unset", "%v", err)
	}
	return &Ctx{Cfg: cfg, Hypr: inst, Driver: inst.Driver()}, nil
}
