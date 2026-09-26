// Package hotkey 解析全局快捷键，不依赖 Win32，便于跨平台验证配置。
package hotkey

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	ModAlt     uint32 = 0x0001
	ModControl uint32 = 0x0002
	ModShift   uint32 = 0x0004
	ModWin     uint32 = 0x0008
)

type Binding struct {
	Modifiers uint32
	Key       uint32
	Label     string
}

// Parse 支持 Ctrl/Alt/Shift/Win 加字母、数字、方向键、翻页键、Home/End、F1..F12、Space。
func Parse(value string) (Binding, error) {
	var binding Binding
	var keyLabel string
	for _, raw := range strings.Split(value, "+") {
		part := strings.ToUpper(strings.TrimSpace(raw))
		if part == "" {
			return Binding{}, fmt.Errorf("快捷键不能为空，也不能包含空按键")
		}
		var modifier uint32
		switch part {
		case "CTRL", "CONTROL":
			modifier = ModControl
		case "ALT":
			modifier = ModAlt
		case "SHIFT":
			modifier = ModShift
		case "WIN", "WINDOWS":
			modifier = ModWin
		}
		if modifier != 0 {
			if binding.Modifiers&modifier != 0 {
				return Binding{}, fmt.Errorf("快捷键重复使用修饰键 %s", raw)
			}
			binding.Modifiers |= modifier
			continue
		}
		if binding.Key != 0 {
			return Binding{}, fmt.Errorf("快捷键只能包含一个主按键")
		}
		key, label, ok := parseKey(part)
		if !ok {
			return Binding{}, fmt.Errorf("不支持的快捷键按键: %s", raw)
		}
		binding.Key, keyLabel = key, label
	}
	if binding.Key == 0 {
		return Binding{}, fmt.Errorf("快捷键缺少主按键")
	}
	labels := make([]string, 0, 5)
	for _, modifier := range []struct {
		bit   uint32
		label string
	}{{ModControl, "Ctrl"}, {ModAlt, "Alt"}, {ModShift, "Shift"}, {ModWin, "Win"}} {
		if binding.Modifiers&modifier.bit != 0 {
			labels = append(labels, modifier.label)
		}
	}
	binding.Label = strings.Join(append(labels, keyLabel), "+")
	return binding, nil
}

func parseKey(value string) (uint32, string, bool) {
	if len(value) == 1 && (value[0] >= 'A' && value[0] <= 'Z' || value[0] >= '0' && value[0] <= '9') {
		return uint32(value[0]), value, true
	}
	keys := map[string]struct {
		code  uint32
		label string
	}{
		"LEFT": {0x25, "Left"}, "UP": {0x26, "Up"}, "RIGHT": {0x27, "Right"}, "DOWN": {0x28, "Down"},
		"PGUP": {0x21, "PgUp"}, "PAGEUP": {0x21, "PgUp"}, "PGDN": {0x22, "PgDn"}, "PAGEDOWN": {0x22, "PgDn"},
		"HOME": {0x24, "Home"}, "END": {0x23, "End"}, "SPACE": {0x20, "Space"}, "SPACEBAR": {0x20, "Space"},
	}
	if key, ok := keys[value]; ok {
		return key.code, key.label, true
	}
	if strings.HasPrefix(value, "F") {
		if number, err := strconv.Atoi(value[1:]); err == nil && number >= 1 && number <= 12 {
			return uint32(0x70 + number - 1), "F" + strconv.Itoa(number), true
		}
	}
	return 0, "", false
}

// Validate 检测本应用内重复组合键；系统中的占用仍由 RegisterHotKey 返回错误。
func Validate(values map[string]string) error {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	seen := make(map[uint64]string)
	for _, name := range names {
		binding, err := Parse(values[name])
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		identity := uint64(binding.Modifiers)<<32 | uint64(binding.Key)
		if previous, exists := seen[identity]; exists {
			return fmt.Errorf("快捷键 %s 同时用于 %s 和 %s，请改用不同组合键", binding.Label, previous, name)
		}
		seen[identity] = name
	}
	return nil
}
