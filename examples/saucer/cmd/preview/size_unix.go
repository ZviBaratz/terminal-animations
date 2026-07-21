//go:build unix

package main

import (
	"syscall"
	"unsafe"
)

// terminalSize returns the terminal's (cols, rows) for the given fd via the
// TIOCGWINSZ ioctl. ok is false when fd is not a terminal (e.g. a pipe).
func terminalSize(fd uintptr) (w, h int, ok bool) {
	var ws struct{ Row, Col, Xpixel, Ypixel uint16 }
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		fd,
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&ws)),
	)
	if errno != 0 || ws.Col == 0 || ws.Row == 0 {
		return 0, 0, false
	}
	return int(ws.Col), int(ws.Row), true
}
