package ui

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func (a *App) watchTheme() {
	fd, err := unix.InotifyInit1(unix.IN_CLOEXEC | unix.IN_NONBLOCK)
	if err != nil {
		return
	}
	file := os.NewFile(uintptr(fd), "ryomarket-theme")
	defer file.Close()
	wall, shell, theme := themePaths()
	watched := map[string]bool{}
	for _, path := range []string{wall, shell, theme} {
		dir := filepath.Dir(path)
		if watched[dir] {
			continue
		}
		if _, err := unix.InotifyAddWatch(fd, dir, unix.IN_CLOSE_WRITE|unix.IN_MOVED_TO|unix.IN_DELETE|unix.IN_CREATE); err == nil {
			watched[dir] = true
		}
	}
	if len(watched) == 0 {
		return
	}
	buffer := make([]byte, 4096)
	for {
		if _, err := file.Read(buffer); err != nil {
			return
		}
		a.win.Invalidate()
	}
}
