//go:build windows

package app

import (
	"os"
	"syscall"
	"time"
)

func fileCreationTime(_ string, info os.FileInfo) time.Time {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return info.ModTime()
	}
	return time.Unix(0, data.CreationTime.Nanoseconds())
}
