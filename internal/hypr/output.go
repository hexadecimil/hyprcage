package hypr

// CreateHeadless adds a fake output (hyprctl output create headless NAME).
// Declare its mode, position and scale, and its workspace rule, BEFORE
// calling this (cahier §4.2 step 3).
func (i *Instance) CreateHeadless(name string) error {
	return i.Command("output create headless " + name)
}

// RemoveOutput removes a fake output. Verified in Hyprland 0.56.2 (cahier
// §4.3): this warps the human's cursor and migrates the output's workspaces
// to a real monitor; the caller must capture and restore the human's state.
func (i *Instance) RemoveOutput(name string) error {
	return i.Command("output remove " + name)
}
