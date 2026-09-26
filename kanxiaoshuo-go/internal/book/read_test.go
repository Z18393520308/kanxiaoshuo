package book

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"
)

func TestDecodeEncodings(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
		want string
	}{
		{"utf8", []byte("中文😀"), "中文😀"},
		{"bom", append([]byte{0xef, 0xbb, 0xbf}, []byte("中文")...), "中文"},
		{"gbk", []byte{0xd6, 0xd0, 0xce, 0xc4}, "中文"},
		{"utf16le", []byte{0xff, 0xfe, 0x2d, 0x4e, 0x87, 0x65, 0x3d, 0xd8, 0x00, 0xde}, "中文😀"},
		{"utf16be", []byte{0xfe, 0xff, 0x4e, 0x2d, 0x65, 0x87, 0xd8, 0x3d, 0xde, 0x00}, "中文😀"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := decodeText(c.raw)
			if err != nil || got != c.want || !utf8.ValidString(got) {
				t.Fatalf("got %q, %v; want %q", got, err, c.want)
			}
		})
	}
}

func TestMalformedBOMEncoding(t *testing.T) {
	for _, raw := range [][]byte{{0xff, 0xfe, 1}, {0xef, 0xbb, 0xbf, 0xff}} {
		if _, err := decodeText(raw); err == nil {
			t.Fatalf("expected decode error for %x", raw)
		}
	}
}

func TestLoadPreservesParagraphs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "novel.txt")
	if err := os.WriteFile(path, []byte("第一段\r\n\r\n第二段\n第三段\r结尾\x00后文"), 0600); err != nil {
		t.Fatal(err)
	}
	last := -1
	got, err := LoadWithProgress(path, func(p int) {
		if p < last || p > 100 {
			t.Fatalf("bad progress %d after %d", p, last)
		}
		last = p
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "第一段\r\n\r\n第二段\r\n第三段\r\n结尾�后文"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if last != 100 {
		t.Fatalf("last progress %d", last)
	}
}
