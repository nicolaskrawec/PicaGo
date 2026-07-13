//go:build windows

package app

import (
	"image"
	"syscall"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	swHide                = 0
	swShow                = 5
	srccopy               = 0x00CC0020
	captureblt            = 0x40000000
	dibRGBColors          = 0
	monitorDefaultNearest = 2
)

type rect struct{ left, top, right, bottom int32 }

type monitorInfo struct {
	size    uint32
	monitor rect
	work    rect
	flags   uint32
}

type bitmapInfoHeader struct {
	size          uint32
	width         int32
	height        int32
	planes        uint16
	bitCount      uint16
	compression   uint32
	sizeImage     uint32
	xPelsPerMeter int32
	yPelsPerMeter int32
	clrUsed       uint32
	clrImportant  uint32
}

type bitmapInfo struct {
	header bitmapInfoHeader
	colors [1]uint32
}

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	gdi32               = syscall.NewLazyDLL("gdi32.dll")
	dwmapi              = syscall.NewLazyDLL("dwmapi.dll")
	getCurrentProcessID = kernel32.NewProc("GetCurrentProcessId")
	enumWindows         = user32.NewProc("EnumWindows")
	getForegroundWindow = user32.NewProc("GetForegroundWindow")
	getWindowProcessID  = user32.NewProc("GetWindowThreadProcessId")
	isWindowVisible     = user32.NewProc("IsWindowVisible")
	showWindow          = user32.NewProc("ShowWindow")
	setForegroundWindow = user32.NewProc("SetForegroundWindow")
	monitorFromWindow   = user32.NewProc("MonitorFromWindow")
	getMonitorInfo      = user32.NewProc("GetMonitorInfoW")
	getDC               = user32.NewProc("GetDC")
	releaseDC           = user32.NewProc("ReleaseDC")
	getDIBits           = gdi32.NewProc("GetDIBits")
	createCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	createCompatibleBmp = gdi32.NewProc("CreateCompatibleBitmap")
	selectObject        = gdi32.NewProc("SelectObject")
	bitBlt              = gdi32.NewProc("BitBlt")
	deleteObject        = gdi32.NewProc("DeleteObject")
	deleteDC            = gdi32.NewProc("DeleteDC")
	dwmFlush            = dwmapi.NewProc("DwmFlush")
)

func captureDesktopBackdrop() *ebiten.Image {
	hwnd := applicationWindow()
	hideWindow := true
	if hwnd == 0 {
		// Capturing without hiding is less ideal because PicaGo may appear in
		// the screenshot, but it is safer than touching another application.
		hwnd, _, _ = getForegroundWindow.Call()
		hideWindow = false
	}
	if hwnd == 0 {
		return nil
	}

	monitor, _, _ := monitorFromWindow.Call(hwnd, monitorDefaultNearest)
	if monitor == 0 {
		return nil
	}
	info := monitorInfo{size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ret, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ret == 0 {
		return nil
	}

	width := int(info.monitor.right - info.monitor.left)
	height := int(info.monitor.bottom - info.monitor.top)
	if width <= 0 || height <= 0 {
		return nil
	}

	if hideWindow {
		// Hide PicaGo so the capture contains what is behind it.
		showWindow.Call(hwnd, swHide)
		dwmFlush.Call()
		defer func() {
			showWindow.Call(hwnd, swShow)
			setForegroundWindow.Call(hwnd)
		}()
	}

	screenDC, _, _ := getDC.Call(0)
	if screenDC == 0 {
		return nil
	}
	defer releaseDC.Call(0, screenDC)

	memDC, _, _ := createCompatibleDC.Call(screenDC)
	if memDC == 0 {
		return nil
	}
	defer deleteDC.Call(memDC)

	bmp, _, _ := createCompatibleBmp.Call(screenDC, uintptr(width), uintptr(height))
	if bmp == 0 {
		return nil
	}
	defer deleteObject.Call(bmp)

	previous, _, _ := selectObject.Call(memDC, bmp)
	if ret, _, _ := bitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height), screenDC,
		uintptr(info.monitor.left), uintptr(info.monitor.top), srccopy|captureblt); ret == 0 {
		selectObject.Call(memDC, previous)
		return nil
	}
	// GetDIBits requires the bitmap not to be selected into a device context.
	selectObject.Call(memDC, previous)

	pixels := make([]byte, width*height*4)
	bi := bitmapInfo{header: bitmapInfoHeader{
		size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), width: int32(width), height: -int32(height),
		planes: 1, bitCount: 32,
	}}
	if ret, _, _ := getDIBits.Call(screenDC, bmp, 0, uintptr(height), uintptr(unsafe.Pointer(&pixels[0])),
		uintptr(unsafe.Pointer(&bi)), dibRGBColors); ret == 0 {
		return nil
	}

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(pixels); i += 4 {
		rgba.Pix[i+0] = pixels[i+2]
		rgba.Pix[i+1] = pixels[i+1]
		rgba.Pix[i+2] = pixels[i+0]
		rgba.Pix[i+3] = 255
	}
	return ebiten.NewImageFromImage(rgba)
}

func applicationWindow() uintptr {
	processID, _, _ := getCurrentProcessID.Call()
	if foreground, _, _ := getForegroundWindow.Call(); foreground != 0 && windowBelongsToProcess(foreground, processID) {
		return foreground
	}

	var found uintptr

	callback := syscall.NewCallback(func(hwnd uintptr, _ uintptr) uintptr {
		if visible, _, _ := isWindowVisible.Call(hwnd); visible == 0 {
			return 1
		}
		if windowBelongsToProcess(hwnd, processID) {
			found = hwnd
			return 0
		}
		return 1
	})
	enumWindows.Call(callback, 0)
	return found
}

func windowBelongsToProcess(hwnd, processID uintptr) bool {
	var windowProcessID uint32
	getWindowProcessID.Call(hwnd, uintptr(unsafe.Pointer(&windowProcessID)))
	return uintptr(windowProcessID) == processID
}
