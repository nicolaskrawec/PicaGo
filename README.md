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
- Smooth cursor-centered zoom, 100% view, fit-to-window and mouse panning.
- Animated clockwise and counter-clockwise rotation.
- Animated horizontal and vertical mirroring.
- Borderless fullscreen with discreet corner and bottom-edge actions.
- Compare two images with vertical, horizontal or circular masks.
- Reverse which side or area reveals the comparison image.
- Optional soft mask edge, image shadow and slider/image synchronization.
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

Use the left and right arrows, or the bottom action bar, to browse neighboring
images in the same directory.

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
- Bottom bar: previous image with left click or wheel up.
- Bottom bar: next image with right click or wheel down.

These controls are intentionally lightweight so the viewer can stay mostly chrome-free.

## Controls

### Mouse

- `Left drag` on the image: pan.
- `Mouse wheel`: zoom in and out around the cursor.
- `Left drag` on the comparison slider: move the split.
- `Shift + mouse wheel` in circular mode: resize the circular mask.
- `Ctrl + mouse wheel` in comparison mode: change the mask opacity.
- `Double click` on the image: toggle fullscreen / restore window.
- `Drag and drop`: load one or two images, or the first image in a folder.
- `Top-left + left click/wheel up`: rotate 90 degrees counter-clockwise.
- `Top-left + right click/wheel down`: rotate 90 degrees clockwise.
- `Top-right + left click`: quit.
- `Bottom bar + left click/wheel up`: previous image.
- `Bottom bar + right click/wheel down`: next image.

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
- `F1`: toggle help overlay.
- `F11`: toggle borderless fullscreen.
- `H`: horizontal split comparison.
- `V`: vertical split comparison.
- Press `H` or `V` again: reverse the split direction.
- `C`: circular comparison mask; press again to invert it.
- `B`: toggle the soft/blurred comparison edge.
- `L`: toggle slider sync with image movement.
- `S`: toggle the image shadow.
- Hold `1`: show image A only.
- Hold `2`: show image B only.
- `Esc`: quit.

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

## Technical Notes

PicaGo is written in Go and uses
[Ebitengine](https://ebitengine.org/) for its desktop rendering and input loop.
The build scripts inject version information from Git and can produce Windows,
Linux and macOS binaries for AMD64 and ARM64.
