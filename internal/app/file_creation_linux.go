//go:build linux

package app

import (
	"os"
	"time"

	"golang.org/x/sys/unix"
)

func fileCreationTime(path string, info os.FileInfo) time.Time {
	var stat unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path, unix.AT_STATX_SYNC_AS_STAT, unix.STATX_BTIME, &stat); err == nil && stat.Mask&unix.STATX_BTIME != 0 {
		return time.Unix(stat.Btime.Sec, int64(stat.Btime.Nsec))
	}
	return info.ModTime()
}
