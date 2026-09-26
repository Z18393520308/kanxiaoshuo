//go:build windows

package reader

import (
	"testing"
	"unsafe"
)

func TestTextMetricWMatchesWindowsABI(t *testing.T) {
	if n := unsafe.Sizeof(textMetricW{}); n != 60 {
		t.Fatalf("TEXTMETRICW size %d, want 60", n)
	}
	if n := unsafe.Offsetof(textMetricW{}.FirstChar); n != 44 {
		t.Fatalf("FirstChar offset %d, want 44", n)
	}
	if n := unsafe.Offsetof(textMetricW{}.Italic); n != 52 {
		t.Fatalf("Italic offset %d, want 52", n)
	}
}
