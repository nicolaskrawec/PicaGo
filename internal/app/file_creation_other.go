//go:build !windows && !linux && !darwin

package app

import (
	"os"
	"time"
)

func fileCreationTime(_ string, info os.FileInfo) time.Time {
	return info.ModTime()
}
