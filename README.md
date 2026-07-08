# PicaGo

PicaGo is a fast desktop image viewer written in Go with [Ebitengine](https://ebitengine.org/).

The goal of the project is simple: bring back the kind of lightweight, immediate image browsing experience that felt great in older desktop tools, with a strong focus on quick comparison workflows. It is especially inspired by the old Picasa image viewer: open an image instantly, move around fluidly, jump through neighboring files, and compare two images without friction.

## What It Does

PicaGo is designed for:

- quickly opening and inspecting images
- moving through images in the same folder
- comparing two images with an interactive split slider
- zooming and panning with minimal UI overhead
- keeping the viewer responsive and uncluttered

## Features

- Fast single-image viewing.
- Drag and drop support to load images directly into the viewer.
- Folder navigation with previous/next image shortcuts.
- Fit-to-window behavior for quick reset and clean framing.
- Smooth zooming centered around the mouse cursor.
- Pan navigation by dragging the image.
- Two-image comparison mode.
- Vertical or horizontal split comparison.
- Reversible comparison direction.
- Slider line that fades in near interaction and stays visible at image edges.
- Optional slider sync with image movement.
- Borderless fullscreen / maximized viewing mode.
- Corner actions in fullscreen mode for fast navigation and close.
- Windows executable icon integration during build.
- Automatic version injection from Git metadata.

## Comparison Mode

When a second image is loaded, PicaGo can switch to a comparison workflow:

- Image A stays as the base image.
- Image B is revealed through a movable split slider.
- The slider can be vertical or horizontal.
- The reveal direction can be flipped.
- The slider can stay synchronized with image movement while panning.
- The slider line appears when you are close to it, fades away when idle, and remains visible when it sits on an image edge.

This makes it useful for before/after checks, export verification, retouch comparison, or visual QA between two versions of the same image.

## Fullscreen Corner Controls

In borderless fullscreen mode, the top corners act as quick actions:

- Top-left corner: previous image with left click.
- Top-left corner: next image with right click or mouse wheel in the corner.
- Top-right corner: close the viewer.

These controls are intentionally lightweight so the viewer can stay mostly chrome-free.

## Controls

### Mouse

- `Left drag` on the image: pan.
- `Mouse wheel`: zoom in and out around the cursor.
- `Left drag` on the comparison slider: move the split.
- `Double click` on the image: toggle fullscreen / restore window.
- `Drag and drop`: load an image.

### Keyboard

- `Left Arrow`: previous image in the current folder.
- `Right Arrow`: next image in the current folder.
- `Up Arrow`: zoom in.
- `Down Arrow`: zoom out.
- `R`: reset to fit-to-window.
- `F1`: toggle help overlay.
- `F11`: toggle borderless fullscreen.
- `H`: horizontal split comparison.
- `V`: vertical split comparison.
- `M`: mirror horizontally.
- `L`: toggle slider sync with image movement.
- `1`: show image A only.
- `2`: show image B only.
- `C`: switch to compare mode.
- `S`: switch to compare mode.
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
- Windows for the PowerShell build script

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
.\scripts\build.ps1 -Targets windows/amd64,linux/amd64,darwin/arm64
.\scripts\build.ps1 -Version v1.3.0
```

For Windows builds, the script uses `rsrc`.
Install it once with:

```powershell
go install github.com/akavel/rsrc@latest
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
