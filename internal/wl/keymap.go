package wl

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// The virtual keyboard needs a keymap before any key event. hyprcage builds
// one itself (the wtype method): one keycode per distinct keysym, which makes
// typing independent of the human's layout and covers the whole of Unicode.

// Fixed keycodes of the four modifier keys, always present in the keymap.
const (
	kcControl = 37
	kcShift   = 50
	kcAlt     = 64
	kcSuper   = 133
)

// maxKeysyms is the number of distinct keysyms one generated keymap holds.
const maxKeysyms = 200

// keyDelay separates a press from its release, and one key from the next.
const keyDelay = 4 * time.Millisecond

// xkb modifier masks, as the modifiers request expects them.
const (
	modShift   = 1
	modControl = 4
	modMod1    = 8 // alt
	modMod4    = 64
)

// keymapFormatXKBV1 is wl_keyboard.keymap_format.xkb_v1.
const keymapFormatXKBV1 = 1

func reservedKeycode(kc int) bool {
	return kc == kcControl || kc == kcShift || kc == kcAlt || kc == kcSuper
}

// keycodeForSlot returns the xkb keycode of the slot-th (1 based) generated
// key: keycodes start at 9 and skip the four modifier keycodes.
func keycodeForSlot(slot int) int {
	n := 0
	for kc := 9; kc <= 255; kc++ {
		if reservedKeycode(kc) {
			continue
		}
		n++
		if n == slot {
			return kc
		}
	}
	return 0
}

// keymap is a generated xkb keymap: an ordered set of keysym names.
type keymap struct {
	order []string
	index map[string]int // keysym name -> 1 based slot
}

func newKeymap() *keymap {
	return &keymap{index: make(map[string]int)}
}

// add reserves a slot for a keysym, returning its slot and false when the
// keymap is full.
func (k *keymap) add(sym string) (int, bool) {
	if slot, ok := k.index[sym]; ok {
		return slot, true
	}
	if len(k.order) >= maxKeysyms {
		return 0, false
	}
	k.order = append(k.order, sym)
	slot := len(k.order)
	k.index[sym] = slot
	return slot, true
}

// keycode returns the xkb keycode of a keysym already in the keymap.
func (k *keymap) keycode(sym string) int {
	slot, ok := k.index[sym]
	if !ok {
		return 0
	}
	return keycodeForSlot(slot)
}

// render writes the keymap in the xkb text format.
func (k *keymap) render() string {
	var b strings.Builder
	b.WriteString("xkb_keymap {\n")
	b.WriteString("xkb_keycodes {\n")
	b.WriteString("minimum = 8;\n")
	b.WriteString("maximum = 255;\n")
	for i := range k.order {
		fmt.Fprintf(&b, "<K%d> = %d;\n", i+1, keycodeForSlot(i+1))
	}
	fmt.Fprintf(&b, "<LCTL> = %d;\n", kcControl)
	fmt.Fprintf(&b, "<LFSH> = %d;\n", kcShift)
	fmt.Fprintf(&b, "<LALT> = %d;\n", kcAlt)
	fmt.Fprintf(&b, "<LWIN> = %d;\n", kcSuper)
	b.WriteString("};\n")
	b.WriteString("xkb_types { include \"complete\" };\n")
	b.WriteString("xkb_compatibility { include \"complete\" };\n")
	b.WriteString("xkb_symbols {\n")
	for i, sym := range k.order {
		fmt.Fprintf(&b, "key <K%d> { [ %s ] };\n", i+1, sym)
	}
	b.WriteString("key <LCTL> { [ Control_L ] };\n")
	b.WriteString("key <LFSH> { [ Shift_L ] };\n")
	b.WriteString("key <LALT> { [ Alt_L ] };\n")
	b.WriteString("key <LWIN> { [ Super_L ] };\n")
	b.WriteString("modifier_map Control { <LCTL> };\n")
	b.WriteString("modifier_map Shift { <LFSH> };\n")
	b.WriteString("modifier_map Mod1 { <LALT> };\n")
	b.WriteString("modifier_map Mod4 { <LWIN> };\n")
	b.WriteString("};\n")
	b.WriteString("};\n")
	return b.String()
}

// runtimeTempFile creates an unlinked file in $XDG_RUNTIME_DIR (or the system
// temporary directory), suitable for passing as a file descriptor.
func runtimeTempFile(pattern string) (*os.File, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	var f *os.File
	var err error
	if dir != "" {
		f, err = os.CreateTemp(dir, pattern)
	}
	if dir == "" || err != nil {
		f, err = os.CreateTemp(os.TempDir(), pattern)
		if err != nil {
			return nil, fmt.Errorf("wl: temporary file: %w", err)
		}
	}
	// The compositor only ever sees the descriptor.
	os.Remove(f.Name())
	return f, nil
}

// keyboardErr explains why there is no virtual keyboard.
func (c *Client) keyboardErr() error {
	if c.vkMgr == 0 {
		return missingErr(ifaceZwpVirtualKeyboardManagerV1)
	}
	return missingErr(ifaceWlSeat)
}

// sendKeymap uploads a keymap on the virtual keyboard. Re-uploading on the
// same object is legitimate (it is how wtype handles long texts).
func (c *Client) sendKeymap(km *keymap) error {
	if c.keyboard == 0 {
		return c.keyboardErr()
	}
	data := km.render()
	f, err := runtimeTempFile("hyprcage-keymap-*")
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append([]byte(data), 0)); err != nil {
		return fmt.Errorf("wl: writing the keymap: %w", err)
	}
	c.send(c.keyboard, reqZwpVirtualKeyboardV1Keymap, uint32(keymapFormatXKBV1), int(f.Fd()), uint32(len(data)+1))
	return c.flush()
}

// keysymForRune maps one character to the keysym name used in the keymap.
func keysymForRune(r rune) string {
	switch r {
	case '\n':
		return "Return"
	case '\t':
		return "Tab"
	}
	return fmt.Sprintf("U%04X", r)
}

// Type types arbitrary Unicode text through the virtual keyboard.
func (c *Client) Type(text string) error {
	if c.keyboard == 0 {
		return c.keyboardErr()
	}
	runes := []rune(text)
	for i := 0; i < len(runes); {
		km := newKeymap()
		codes := make([]int, 0, maxKeysyms)
		j := i
		for ; j < len(runes); j++ {
			slot, ok := km.add(keysymForRune(runes[j]))
			if !ok {
				break
			}
			codes = append(codes, keycodeForSlot(slot))
		}
		if err := c.sendKeymap(km); err != nil {
			return err
		}
		for _, kc := range codes {
			if err := c.tapKey(kc); err != nil {
				return err
			}
		}
		if err := c.Roundtrip(); err != nil {
			return err
		}
		i = j
	}
	return nil
}

// tapKey presses and releases one xkb keycode. The protocol carries evdev
// codes, which are the xkb keycode minus 8.
func (c *Client) tapKey(keycode int) error {
	if err := c.keyEvent(keycode, true); err != nil {
		return err
	}
	time.Sleep(keyDelay)
	if err := c.keyEvent(keycode, false); err != nil {
		return err
	}
	time.Sleep(keyDelay)
	return nil
}

func (c *Client) keyEvent(keycode int, pressed bool) error {
	var state uint32
	if pressed {
		state = 1
	}
	c.send(c.keyboard, reqZwpVirtualKeyboardV1Key, c.now(), uint32(keycode-8), state)
	return c.flush()
}

// comboModifier is one modifier of a key combination.
type comboModifier struct {
	mask    uint32
	keycode int
}

var comboModifiers = map[string]comboModifier{
	"ctrl":    {modControl, kcControl},
	"control": {modControl, kcControl},
	"shift":   {modShift, kcShift},
	"alt":     {modMod1, kcAlt},
	"super":   {modMod4, kcSuper},
	"meta":    {modMod4, kcSuper},
	"win":     {modMod4, kcSuper},
	"logo":    {modMod4, kcSuper},
}

// keysymNames is the embedded table of xkbcommon names accepted as the last
// token of a combination.
var keysymNames = []string{
	"Return", "Escape", "Tab", "BackSpace", "Delete", "Insert",
	"Home", "End", "Page_Up", "Page_Down",
	"Left", "Right", "Up", "Down",
	"F1", "F2", "F3", "F4", "F5", "F6", "F7", "F8", "F9", "F10", "F11", "F12",
	"F13", "F14", "F15", "F16", "F17", "F18", "F19", "F20", "F21", "F22", "F23", "F24",
	"space", "plus", "minus", "comma", "period", "slash", "semicolon",
	"apostrophe", "bracketleft", "bracketright", "backslash", "grave", "equal",
	"Menu", "Print", "Pause", "Caps_Lock", "Num_Lock",
	"KP_Enter", "KP_0", "KP_1", "KP_2", "KP_3", "KP_4",
	"KP_5", "KP_6", "KP_7", "KP_8", "KP_9",
}

var keysymByLower = func() map[string]string {
	m := make(map[string]string, len(keysymNames))
	for _, n := range keysymNames {
		m[strings.ToLower(n)] = n
	}
	return m
}()

// keysymForToken resolves the last token of a combination: a single
// character becomes a Unicode keysym, anything else must be a known name.
func keysymForToken(tok string) (string, error) {
	r := []rune(tok)
	if len(r) == 1 {
		return fmt.Sprintf("U%04X", r[0]), nil
	}
	if n, ok := keysymByLower[strings.ToLower(tok)]; ok {
		return n, nil
	}
	return "", fmt.Errorf("wl: unknown key name %q (expected a single character or an xkb keysym name such as Return or F5)", tok)
}

// Key presses and releases a combination such as "ctrl+l" or "Return".
func (c *Client) Key(combo string) error {
	if c.keyboard == 0 {
		return c.keyboardErr()
	}
	tokens := strings.Split(combo, "+")
	for i, t := range tokens {
		tokens[i] = strings.TrimSpace(t)
		if tokens[i] == "" {
			return fmt.Errorf("wl: malformed key combination %q", combo)
		}
	}
	var mods []comboModifier
	var mask uint32
	for _, t := range tokens[:len(tokens)-1] {
		m, ok := comboModifiers[strings.ToLower(t)]
		if !ok {
			return fmt.Errorf("wl: unknown modifier %q in %q (expected ctrl, shift, alt or super)", t, combo)
		}
		mods = append(mods, m)
		mask |= m.mask
	}
	sym, err := keysymForToken(tokens[len(tokens)-1])
	if err != nil {
		return err
	}

	km := newKeymap()
	if _, ok := km.add(sym); !ok {
		return fmt.Errorf("wl: cannot map keysym %s", sym)
	}
	if err := c.sendKeymap(km); err != nil {
		return err
	}

	if mask != 0 {
		c.send(c.keyboard, reqZwpVirtualKeyboardV1Modifiers, mask, uint32(0), uint32(0), uint32(0))
		if err := c.flush(); err != nil {
			return err
		}
	}
	for _, m := range mods {
		if err := c.keyEvent(m.keycode, true); err != nil {
			return err
		}
		time.Sleep(keyDelay)
	}
	if err := c.tapKey(km.keycode(sym)); err != nil {
		return err
	}
	for i := len(mods) - 1; i >= 0; i-- {
		if err := c.keyEvent(mods[i].keycode, false); err != nil {
			return err
		}
		time.Sleep(keyDelay)
	}
	if mask != 0 {
		c.send(c.keyboard, reqZwpVirtualKeyboardV1Modifiers, uint32(0), uint32(0), uint32(0), uint32(0))
	}
	return c.Roundtrip()
}
