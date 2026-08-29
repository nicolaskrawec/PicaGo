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
	createCompatibleDC  = gdi32.NewProc("CreateCompatibleDC")
	createDIBSection    = gdi32.NewProc("CreateDIBSection")
	selectObject        = gdi32.NewProc("SelectObject")
	bitBlt              = gdi32.NewProc("BitBlt")
	deleteObject        = gdi32.NewProc("DeleteObject")
	deleteDC            = gdi32.NewProc("DeleteDC")
)

func captureDesktopBackdrop() *ebiten.Image {
	hwnd := applicationWindow()
	hideWindow := true
	debugf("desktop capture: application hwnd=%#x", hwnd)
	if hwnd == 0 {
		// Capturing without hiding is less ideal because PicaGo may appear in
		// the screenshot, but it is safer than touching another application.
		hwnd, _, _ = getForegroundWindow.Call()
		hideWindow = false
		debugf("desktop capture: no application window found, fallback foreground hwnd=%#x", hwnd)
	}
	if hwnd == 0 {
		debugf("desktop capture: failed, no window handle")
		return nil
	}

	monitor, _, _ := monitorFromWindow.Call(hwnd, monitorDefaultNearest)
	if monitor == 0 {
		debugf("desktop capture: MonitorFromWindow failed: %v", syscall.GetLastError())
		return nil
	}
	info := monitorInfo{size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ret, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ret == 0 {
		debugf("desktop capture: GetMonitorInfoW failed: %v", syscall.GetLastError())
		return nil
	}

	width := int(info.monitor.right - info.monitor.left)
	height := int(info.monitor.bottom - info.monitor.top)
	if width <= 0 || height <= 0 {
		debugf("desktop capture: invalid monitor rectangle: left=%d top=%d right=%d bottom=%d", info.monitor.left, info.monitor.top, info.monitor.right, info.monitor.bottom)
		return nil
	}
	debugf("desktop capture: monitor=%#x rect=(%d,%d)-(%d,%d) size=%dx%d hide=%v", monitor, info.monitor.left, info.monitor.top, info.monitor.right, info.monitor.bottom, width, height, hideWindow)

	if hideWindow {
		// Hide PicaGo so the capture contains what is behind it.
		showWindow.Call(hwnd, swHide)
		// Do not call DwmFlush here. This function runs from Ebiten's update
		// loop, and waiting synchronously for the compositor can deadlock (or
		// appear to freeze the application) while the window is transitioning
		// from windowed to fullscreen.
		defer func() {
			showWindow.Call(hwnd, swShow)
			setForegroundWindow.Call(hwnd)
		}()
	}

	screenDC, _, _ := getDC.Call(0)
	if screenDC == 0 {
		debugf("desktop capture: GetDC(NULL) failed: %v", syscall.GetLastError())
		return nil
	}
	defer releaseDC.Call(0, screenDC)

	memDC, _, _ := createCompatibleDC.Call(screenDC)
	if memDC == 0 {
		debugf("desktop capture: CreateCompatibleDC failed: %v", syscall.GetLastError())
		return nil
	}
	defer deleteDC.Call(memDC)

	var bitmapPixels unsafe.Pointer
	bi := bitmapInfo{header: bitmapInfoHeader{
		size: uint32(unsafe.Sizeof(bitmapInfoHeader{})), width: int32(width), height: -int32(height),
		planes: 1, bitCount: 32,
	}}
	bmp, _, _ := createDIBSection.Call(screenDC, uintptr(unsafe.Pointer(&bi)), dibRGBColors, uintptr(unsafe.Pointer(&bitmapPixels)), 0, 0)
	if bmp == 0 {
		debugf("desktop capture: CreateDIBSection failed: %v", syscall.GetLastError())
		return nil
	}
	defer deleteObject.Call(bmp)

	previous, _, _ := selectObject.Call(memDC, bmp)
	if ret, _, _ := bitBlt.Call(memDC, 0, 0, uintptr(width), uintptr(height), screenDC,
		uintptr(info.monitor.left), uintptr(info.monitor.top), srccopy|captureblt); ret == 0 {
		selectObject.Call(memDC, previous)
		debugf("desktop capture: BitBlt failed: %v", syscall.GetLastError())
		return nil
	}
	selectObject.Call(memDC, previous)

	if bitmapPixels == nil {
		debugf("desktop capture: CreateDIBSection returned no pixel buffer")
		return nil
	}
	pixelCount := width * height * 4
	pixels := make([]byte, pixelCount)
	copy(pixels, unsafe.Slice((*byte)(bitmapPixels), pixelCount))

	rgba := image.NewRGBA(image.Rect(0, 0, width, height))
	for i := 0; i < len(pixels); i += 4 {
		rgba.Pix[i+0] = pixels[i+2]
		rgba.Pix[i+1] = pixels[i+1]
		rgba.Pix[i+2] = pixels[i+0]
		rgba.Pix[i+3] = 255
	}
	debugf("desktop capture: success, image=%dx%d", width, height)
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
