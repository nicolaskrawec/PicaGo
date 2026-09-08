# PicaGo

**A fast, fluid and distraction-free image viewer built for people who want to
look at images, not fight with an interface.**

Inspired by the immediacy of the classic Picasa viewer, PicaGo opens images
quickly, navigates through folders smoothly and offers a powerful two-image
comparison workflow without covering the picture with toolbars.

Use it for everyday browsing, checking a retouch against its original,
comparing exports, spotting compression differences, or performing fast visual
QA. The interface stays out of the way until you need it.

## Features

- Fast image opening and fluid folder navigation.
- Open an image from the command line or drop images and folders into the window.
- Navigate through every supported image in the current folder.
- Browse a centered bottom strip of asynchronously loaded, memory-bounded
  thumbnails; it appears near the bottom edge, hides after mouse inactivity,
  and lets you click a neighboring thumbnail to open it directly.
- Smooth cursor-centered zoom, 100% view, fit-to-window and mouse panning.
- Animated clockwise and counter-clockwise rotation.
- Animated horizontal and vertical mirroring.
- Borderless fullscreen with discreet corner and thumbnail controls.
- Compare two images with vertical, horizontal or circular masks.
- Reverse which side or area reveals the comparison image.
- Image shadow and slider/image synchronization.
- Compare images with different dimensions and aspect ratios.
- Drag and drop onto the left or right half of the window to replace image A or
  B directly.
- Persistent fit-to-window mode that follows window resizing and fullscreen
  transitions.

## Getting Started

Open an image by passing its path to PicaGo:

```text
PicaGo photo.jpg
```

You can also start PicaGo without an argument and drag an image into the
window. With an image already loaded, drop on the left half to replace image A
or on the right half to replace image B. Dropping a second image activates the
comparison workflow. Dropping multiple images uses the first two supported
files as images A and B, while dropping a folder opens its first supported
image.

Use the left and right arrows, or the thumbnail strip, to browse neighboring
images in the same directory. The current image stays centered in the strip,
with previous images on the left and following images on the right.

## Comparison Mode

When a second image is loaded, PicaGo switches to a shared comparison view:

- Image A stays as the base image.
- Image B is revealed through a movable split slider.
- The slider can be vertical or horizontal.
- The reveal direction can be flipped.
- `V` selects a vertical split and `H` a horizontal split. Press the active key
  again to reverse the revealed side.
- `C` selects a circular mask. Press it again to exchange the inside and outside.
- The split slider can stay synchronized with image movement while panning.
- The slider line appears when you are close to it, fades away when idle, and remains visible when it sits on an image edge.
- Both images, the mask and the slider rotate together.
- Hold `1` or `2` to inspect either source image on its own.

This makes it useful for before/after checks, export verification, retouch comparison, or visual QA between two versions of the same image.

## Screen-edge Controls

PicaGo keeps its interface minimal by exposing actions at the edges of the
window:

- Top-left corner: rotate counter-clockwise with left click or wheel up.
- Top-left corner: rotate clockwise with right click or wheel down.
- Top-right corner: close the viewer.

These controls are intentionally lightweight so the viewer can stay mostly chrome-free.

## Controls

### Mouse

- `Left drag` on the image: pan.
- `Mouse wheel`: zoom in and out around the cursor.
- `Left drag` on the comparison slider: move the split.
- `Shift + mouse wheel`: adjust gamma.
- `Alt + mouse wheel`: adjust exposure/brightness.
- `Ctrl + Alt + mouse wheel`: adjust contrast.
- `Ctrl + mouse wheel` in comparison mode: change the mask opacity.
- `Ctrl + Shift + mouse wheel` in circular comparison mode: resize the circular mask.
- `Double click` on the image: enter fullscreen; while fullscreen, toggle
  100% / previous zoom and position.
- `Drag and drop`: load one or two images, or the first image in a folder.
- `Top-left + left click/wheel up`: rotate 90 degrees counter-clockwise.
- `Top-left + right click/wheel down`: rotate 90 degrees clockwise.
- `Top-right + left click`: quit.
- `Thumbnail strip + left click`: open that image directly.

### Keyboard

- `Left Arrow`: previous image in the current folder.
- `Right Arrow`: next image in the current folder.
- `Up Arrow`: zoom in.
- `Down Arrow`: zoom out.
- `Ctrl + Left Arrow`: rotate 90 degrees counter-clockwise.
- `Ctrl + Right Arrow`: rotate 90 degrees clockwise.
- `Shift + Left/Right Arrow`: mirror horizontally.
- `Shift + Up/Down Arrow`: mirror vertically.
- `R`: reset to fit-to-window and keep fit mode active.
- `Z` (`W` on some keyboard layouts): toggle 100% / maximum fit, centered on
  the mouse cursor when switching to 100%.
- Manual panning or zooming leaves persistent fit mode; resizing then keeps the
  chosen view instead of refitting it.
- `F1`: toggle the English help overlay.
- `F11`: toggle borderless fullscreen.
- `H`: horizontal split comparison.
- `V`: vertical split comparison.
- Press `H` or `V` again: reverse the split direction.
- `C`: circular comparison mask; press again to invert it.
- `B`: cycle the background. In fullscreen, this cycles desktop, gray, black
  and white when the desktop background is available. In a window, or without
  desktop capture, it cycles gray, black and white.
- `P`: play/pause a looping slideshow of the images in the current folder.
- `O`: cycle the current folder's image order between name, modification date
  and creation date, in ascending and descending order.
- `L`: toggle slider sync with image movement.
- `S`: toggle the image shadow.
- `Shift + mouse wheel`: adjust the gamma correction of the displayed image;
  in comparison mode, both images are adjusted together.
- `Alt + mouse wheel`: adjust exposure/brightness.
- `Ctrl + Alt + mouse wheel`: adjust contrast.
- `Ctrl + Shift + mouse wheel` in circular comparison mode: resize the circular
  mask.
- Hold `1`: show image A only.
- Hold `2`: show image B only.
- `Esc`: quit.

## Configuration

PicaGo stores its per-user settings in `config.json` in the operating system's
standard configuration directory: `%AppData%\PicaGo` on Windows,
`$XDG_CONFIG_HOME/PicaGo` (usually `~/.config/PicaGo`) on Linux, and
`~/Library/Application Support/PicaGo` on macOS. The legacy `PicaGo.json`
beside the executable is imported automatically on first launch.

The view parameters changed for individual images are stored separately in
`image-views.json` in the same directory. They are restored automatically when
the image is opened again. The file uses compact JSON and indexes settings by
a fast hash of the normalized absolute path; the path itself is not stored.
Images without changed parameters are not kept in that file. Older key formats
are ignored.

The image order selected with `O` is stored per folder in
`folder-settings.json`, alongside `image-views.json`. Folder paths are
normalized and stored only as SHA-256 hashes, never in clear text. Folders
without a saved preference are ordered by name.

```json
{
  "desktopBackground": true,
  "background": "desktop",
  "showShadow": true,
  "slideshowIntervalSeconds": 1,
  "animateOnStart": true,
  "showDebug": false,
  "graphicsLibrary": "auto"
}
```

- `desktopBackground`: enable the captured Windows desktop as a fullscreen
  background option. Press `B` to cycle between it and the gray, black and
  white backgrounds. Set it to `false` to disable desktop capture.
- `background`: last selected background: `desktop`, `gray`, `black` or `white`.
- `showShadow`: show the image shadow.
- `slideshowIntervalSeconds`: number of seconds between two slideshow images.
- `animateOnStart`: animate the initial image fit when opening PicaGo.
- `showDebug`: show the debug/help overlay at startup; `F1` toggles it afterward.
- `graphicsLibrary`: graphics backend requested at startup: `auto`, `opengl` or
  `directx`. The default `auto` uses the platform-preferred backend. The backend
  actually selected is shown in the `F1` overlay.

If the file is missing, PicaGo creates it with these default values.

## Supported Formats

Currently supported formats:

- JPEG
- PNG
- GIF
- WebP
- BMP
- TIFF

## Requirements

- Go installed
- Git installed for automatic build versioning
- PowerShell on Windows, or Bash on Linux, for the build scripts

## Run in Development

From the project root:

```powershell
go run .
```

Or with an image:

```powershell
go run . "C:\path\to\image.jpg"
```

Without arguments, the app opens in a `640x480` window.

## Automated Builds

From the project root:

```powershell
.\scripts\build.ps1
```

On Linux:

```bash
chmod +x ./scripts/build.sh
./scripts/build.sh
```

The script:

- computes the version automatically from Git
- injects that version into `viewergo/internal/app.Version`
- produces multi-platform binaries in `dist/`
- builds Windows binaries without a visible console window
- embeds the `.exe` icon from `internal/assets/icon.ico`

Automatic version examples:

- exact tag on `HEAD`: `v1.2.3`
- commits after a tag: `v1.2.3+4.6839b09`
- no tag yet: `v0.0.0+6839b09`
- local modifications: `.dirty` suffix

Examples:

```powershell
.\scripts\build.ps1 -Targets windows/amd64
.\scripts\build.ps1 -Targets windows/amd64 -Goamd64 v3
.\scripts\build.ps1 -Targets windows/amd64,linux/amd64,darwin/arm64
.\scripts\build.ps1 -Version v1.3.0
```

The `-Goamd64 v3` option builds a Windows AMD64 executable optimized for more
recent processors. It can provide better performance, but the resulting binary
will not run on older CPUs that do not support the required instruction set.
Use the default `v1` for maximum compatibility. This option only applies to
AMD64 targets and accepts `v1`, `v2`, `v3` or `v4`.

Linux/Bash equivalents:

```bash
./scripts/build.sh --targets linux/amd64
./scripts/build.sh --targets windows/amd64 --goamd64 v3
./scripts/build.sh --targets windows/amd64,linux/amd64,darwin/arm64
./scripts/build.sh --version v1.3.0
```

For Windows builds, the script uses `goversioninfo` to embed the icon and the
Windows file properties (`ProductName`, `CompanyName`, `FileDescription`, `FileVersion`,
`ProductVersion` and `OriginalFilename`).
Install it once with:

```powershell
go install github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest
```

The `internal/assets/icon.ico` file is used:

- as the application window icon
- as the Windows `.exe` file icon during build

For `windows/amd64`, the build also writes a stable `dist/PicaGo.exe` file in addition to the versioned artifact, which is useful for packaging and local testing.

## Windows Installer

PicaGo can be packaged as a Windows installer with:

```powershell
.\scripts\build-installer.ps1
```

This script:

- builds `dist/PicaGo.exe`
- copies the default `PicaGo.json` configuration next to the executable
- compiles an Inno Setup installer into `dist/installer`
- installs the app with the embedded icon
- registers PicaGo as a handler for supported image formats

Requirements:

- Inno Setup 6 installed
- `ISCC.exe` available in `PATH` or in the default Inno Setup install directory

Notes about default file opening on Windows 10/11:

- the installer registers PicaGo properly for supported extensions
- Windows may still require a user confirmation in `Default apps`
- the installer includes an optional checkbox to open the Windows default apps settings at the end
## Technical Notes

PicaGo is written in Go and uses
[Ebitengine](https://ebitengine.org/) for its desktop rendering and input loop.
The build scripts inject version information from Git and can produce Windows,
Linux and macOS binaries for AMD64 and ARM64.
