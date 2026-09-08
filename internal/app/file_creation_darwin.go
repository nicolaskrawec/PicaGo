//go:build darwin

package app

import (
	"os"
	"syscall"
	"time"
)

func fileCreationTime(_ string, info os.FileInfo) time.Time {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.ModTime()
	}
	return time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}
