package hotkey

import "testing"

func TestParseSupportedBindings(t *testing.T) {
	for _, tc := range []struct {
		input, label   string
		modifiers, key uint32
	}{
		{"alt + up", "Alt+Up", ModAlt, 0x26},
		{"Shift+Control+a", "Ctrl+Shift+A", ModControl | ModShift, 0x41},
		{"Win+Alt+Ctrl+Shift+9", "Ctrl+Alt+Shift+Win+9", 15, 0x39},
		{"ctrl+PageUp", "Ctrl+PgUp", ModControl, 0x21},
		{"Alt+PgDn", "Alt+PgDn", ModAlt, 0x22},
		{"Alt+PageDown", "Alt+PgDn", ModAlt, 0x22},
		{"Win+Home", "Win+Home", ModWin, 0x24},
		{"End", "End", 0, 0x23},
		{"Ctrl+F1", "Ctrl+F1", ModControl, 0x70},
		{"Shift+F12", "Shift+F12", ModShift, 0x7B},
		{"Alt+Spacebar", "Alt+Space", ModAlt, 0x20},
	} {
		t.Run(tc.input, func(t *testing.T) {
			got, err := Parse(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			if got.Label != tc.label || got.Modifiers != tc.modifiers || got.Key != tc.key {
				t.Fatalf("got %+v, want %+v", got, tc)
			}
		})
	}
}

func TestParseRejectsIncompleteUnsupportedAndAmbiguousBindings(t *testing.T) {
	for _, input := range []string{"", " ", "Ctrl", "Ctrl+", "+A", "Ctrl++A", "Ctrl+A+B", "Ctrl+Ctrl+A", "F0", "F13", "Alt+中文", "Ctrl+!"} {
		t.Run(input, func(t *testing.T) {
			if _, err := Parse(input); err == nil {
				t.Fatalf("accepted %q", input)
			}
		})
	}
}

func TestValidateRecognizesCanonicalConflicts(t *testing.T) {
	if err := Validate(map[string]string{"up": "Alt+Ctrl+PgUp", "down": "control+alt+pageup"}); err == nil {
		t.Fatal("missed same key with aliases/order/case")
	}
	if err := Validate(map[string]string{"up": "Alt+Up", "down": "Alt+Down", "show": "Alt+S"}); err != nil {
		t.Fatal(err)
	}
	if err := Validate(map[string]string{"up": "Alt"}); err == nil {
		t.Fatal("ignored invalid binding")
	}
}
