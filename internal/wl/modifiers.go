package wl

import (
	"fmt"
	"strings"
	"time"
)

// HoldModifiers presses the named modifiers (ctrl, shift, alt, super) and
// keeps them held for the pointer events that follow, e.g. a ctrl+click.
// An empty list releases whatever is held. The four modifier keys are part
// of every generated keymap, including the initial one, so no keymap upload
// is needed here.
func (c *Client) HoldModifiers(mods []string) error {
	if c.keyboard == 0 {
		return c.keyboardErr()
	}
	if len(c.heldMods) > 0 {
		for i := len(c.heldMods) - 1; i >= 0; i-- {
			if err := c.keyEvent(c.heldMods[i].keycode, false); err != nil {
				return err
			}
			time.Sleep(keyDelay)
		}
		c.heldMods = nil
		c.send(c.keyboard, reqZwpVirtualKeyboardV1Modifiers, uint32(0), uint32(0), uint32(0), uint32(0))
		if err := c.flush(); err != nil {
			return err
		}
	}
	if len(mods) == 0 {
		return c.Roundtrip()
	}
	var held []comboModifier
	var mask uint32
	for _, t := range mods {
		m, ok := comboModifiers[strings.ToLower(strings.TrimSpace(t))]
		if !ok {
			return fmt.Errorf("wl: unknown modifier %q (expected ctrl, shift, alt or super)", t)
		}
		held = append(held, m)
		mask |= m.mask
	}
	c.send(c.keyboard, reqZwpVirtualKeyboardV1Modifiers, mask, uint32(0), uint32(0), uint32(0))
	if err := c.flush(); err != nil {
		return err
	}
	for _, m := range held {
		if err := c.keyEvent(m.keycode, true); err != nil {
			return err
		}
		time.Sleep(keyDelay)
	}
	c.heldMods = held
	return c.Roundtrip()
}
