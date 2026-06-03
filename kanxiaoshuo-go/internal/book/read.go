package book

import (
	"bytes"
	"io"
	"os"
	"strings"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// ProgressFn 读取进度回调，percent 为 0–100
type ProgressFn func(percent int)

// Load 读取 TXT 全文（与原 C# 版一致：按行读取后拼接，不保留换行）
func Load(path string) (string, error) {
	return LoadWithProgress(path, nil)
}

// LoadWithProgress 分块读取并报告进度
func LoadWithProgress(path string, onProgress ProgressFn) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	size := fi.Size()

	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var raw []byte
	if size > 0 {
		raw = make([]byte, 0, size)
	} else {
		raw = make([]byte, 0, 4096)
	}

	buf := make([]byte, 64*1024)
	var read int64
	for {
		n, err := f.Read(buf)
		if n > 0 {
			raw = append(raw, buf[:n]...)
			read += int64(n)
			if onProgress != nil && size > 0 {
				p := int(read * 100 / size)
				if p > 100 {
					p = 100
				}
				onProgress(p)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	if onProgress != nil {
		onProgress(100)
	}

	text, err := decodeText(raw)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	b.Grow(len(text))
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimRight(line, "\r")
		b.WriteString(line)
	}
	return b.String(), nil
}

func decodeText(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		return string(raw[3:]), nil
	}
	if isValidUTF8(raw) {
		return string(raw), nil
	}
	r := transform.NewReader(bytes.NewReader(raw), simplifiedchinese.GBK.NewDecoder())
	out, err := io.ReadAll(r)
	if err != nil {
		return string(raw), nil
	}
	return string(out), nil
}

func isValidUTF8(b []byte) bool {
	return bytes.Equal(b, []byte(string(b)))
}
