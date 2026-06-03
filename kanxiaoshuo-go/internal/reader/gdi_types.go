//go:build windows

package reader

const (
	dtLeft      = 0x00000000
	dtWordBreak = 0x00000010
	dtNoPrefix  = 0x00000800
	dtCalcrect  = 0x00000400
	dtTop       = 0x00000008
)

type rect struct {
	Left, Top, Right, Bottom int32
}

var (
	procDrawTextW          = user32.NewProc("DrawTextW")
	procCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	procDeleteDC           = gdi32.NewProc("DeleteDC")
	procSetBkMode          = gdi32.NewProc("SetBkMode")
	procSetTextColor       = gdi32.NewProc("SetTextColor")
)
