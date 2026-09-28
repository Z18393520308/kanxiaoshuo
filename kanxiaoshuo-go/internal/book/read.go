package book

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"golang.org/x/text/encoding/simplifiedchinese"
	"io"
	"os"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// ProgressFn 读取进度回调，percent 为 0–100。
type ProgressFn func(percent int)

// Load 返回统一的 UTF-8 文本。保留段落和历史书签格式，换行统一为 CRLF。
// 阅读位置指向这份规范化文本的 UTF-8 字节位置，与原文件编码无关。
func Load(path string) (string, error) { return LoadWithProgress(path, nil) }

func LoadWithProgress(path string, onProgress ProgressFn) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("请选择普通 TXT 文件")
	}
	var raw bytes.Buffer
	buf := make([]byte, 64*1024)
	var read int64
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			raw.Write(buf[:n])
			read += int64(n)
			if onProgress != nil && fi.Size() > 0 {
				onProgress(min(99, int(read*100/fi.Size())))
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return "", readErr
		}
	}
	text, err := decodeText(raw.Bytes())
	if err != nil {
		return "", err
	}
	// NUL 会截断 Win32 字符串，替换为可见字符，避免后半本书静默消失。
	text = strings.ReplaceAll(text, "\x00", "�")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.ReplaceAll(text, "\n", "\r\n")
	if onProgress != nil {
		onProgress(100)
	}
	return text, nil
}

func decodeText(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	if bytes.HasPrefix(raw, []byte{0xFF, 0xFE}) {
		return decodeUTF16(raw[2:], binary.LittleEndian)
	}
	if bytes.HasPrefix(raw, []byte{0xFE, 0xFF}) {
		return decodeUTF16(raw[2:], binary.BigEndian)
	}
	if bytes.HasPrefix(raw, []byte{0xEF, 0xBB, 0xBF}) {
		raw = raw[3:]
		if !utf8.Valid(raw) {
			return "", fmt.Errorf("UTF-8 文件包含无效字符")
		}
		return string(raw), nil
	}
	if utf8.Valid(raw) {
		return string(raw), nil
	}
	out, err := simplifiedchinese.GBK.NewDecoder().Bytes(raw)
	if err != nil {
		return "", fmt.Errorf("无法按 GBK 解码文件: %w", err)
	}
	return string(out), nil
}

func decodeUTF16(raw []byte, order binary.ByteOrder) (string, error) {
	if len(raw)%2 != 0 {
		return "", fmt.Errorf("UTF-16 文件长度不完整")
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = order.Uint16(raw[i*2:])
	}
	return string(utf16.Decode(units)), nil
}
