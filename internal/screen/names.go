package screen

import (
	"crypto/rand"
	"encoding/hex"
	"regexp"
)

var nameRE = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// ValidateName enforces cahier F1: the name crosses Lua, sh and Hyprland's
// IPC, so only a strict alphabet is accepted.
func ValidateName(name string) error {
	if !nameRE.MatchString(name) {
		return errf(CodeInvalidName, "use lowercase letters, digits and dashes", "%q must match ^[a-z0-9-]{1,32}$", name)
	}
	return nil
}

// NewName returns prefix + 6 random hex characters.
func NewName(prefix string) string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b)
}
