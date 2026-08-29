package app

import (
	"log"
	"os"
	"path/filepath"
)

var debugLog *log.Logger

func initDebugLogging(enabled bool) {
	if !enabled {
		return
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		log.Printf("debug logging: cannot resolve config directory: %v", err)
		return
	}
	path := filepath.Join(dir, appConfigDirectory, "debug.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Printf("debug logging: cannot open %s: %v", path, err)
		return
	}
	debugLog = log.New(file, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	debugLog.Printf("debug logging enabled; log file: %s", path)
}

func debugf(format string, args ...interface{}) {
	if debugLog != nil {
		debugLog.Printf(format, args...)
	}
}
