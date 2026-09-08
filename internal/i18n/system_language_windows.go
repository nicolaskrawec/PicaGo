package i18n

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getUserDefaultLocaleName = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetUserDefaultLocaleName")

func systemLanguage() string {
	// LOCALE_NAME_MAX_LENGTH, including the terminating null character.
	buffer := make([]uint16, 85)
	length, _, _ := getUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(len(buffer)),
	)
	if length != 0 {
		return syscall.UTF16ToString(buffer)
	}
	return languageFromEnvironment()
}
