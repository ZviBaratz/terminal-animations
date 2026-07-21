//go:build !unix

package main

// terminalSize has no portable non-unix implementation here; callers fall back to a
// fixed size. (This preview is a unix terminal tool; the animation itself is portable.)
func terminalSize(fd uintptr) (w, h int, ok bool) { return 0, 0, false }
