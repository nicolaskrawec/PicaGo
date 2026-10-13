package app

import (
	"context"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestSortNavigationImages(t *testing.T) {
	oldest := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	newest := oldest.Add(24 * time.Hour)
	tests := []struct {
		name string
		mode imageSortMode
		want []string
	}{
		{name: "name ascending", mode: imageSortByNameAscending, want: []string{"a.png", "b.png", "c.png"}},
		{name: "name descending", mode: imageSortByNameDescending, want: []string{"c.png", "b.png", "a.png"}},
		{name: "modification ascending", mode: imageSortByModificationDateAscending, want: []string{"a.png", "b.png", "c.png"}},
		{name: "modification descending", mode: imageSortByModificationDateDescending, want: []string{"c.png", "a.png", "b.png"}},
		{name: "creation ascending", mode: imageSortByCreationDateAscending, want: []string{"a.png", "c.png", "b.png"}},
		{name: "creation descending", mode: imageSortByCreationDateDescending, want: []string{"b.png", "a.png", "c.png"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			images := []navigationImage{
				{name: "b.png", modifiedTime: oldest, createdTime: newest},
				{name: "c.png", modifiedTime: newest, createdTime: oldest},
				{name: "a.png", modifiedTime: oldest, createdTime: oldest},
			}
			sortNavigationImages(images, test.mode)
			for i, want := range test.want {
				if images[i].name != want {
					t.Fatalf("image %d = %q, want %q", i, images[i].name, want)
				}
			}
		})
	}
}

func TestFolderSettingsKeyObscuresNormalizedPath(t *testing.T) {
	dir := filepath.Join("some", "private", "pictures")
	key := folderSettingsKey(dir)
	if !isFolderSettingsKey(key) {
		t.Fatalf("folder key %q is invalid", key)
	}
	if strings.Contains(strings.ToLower(key), "pictures") {
		t.Fatalf("folder key exposes the directory name: %q", key)
	}
	if key != folderSettingsKey(filepath.Clean(dir)) {
		t.Fatal("equivalent directory paths produced different keys")
	}
}

func TestImageSortModeFromConfigRejectsUnknownValues(t *testing.T) {
	if got := imageSortModeFromConfig("unknown"); got != imageSortByNameAscending {
		t.Fatalf("unknown sort mode = %v, want name", got)
	}
}

func TestFolderSortModeIsScopedToDirectory(t *testing.T) {
	first := filepath.Join("pictures", "first")
	second := filepath.Join("pictures", "second")
	v := Viewer{folderSortModes: map[string]imageSortMode{
		folderSettingsKey(first): imageSortByCreationDateDescending,
	}}
	if got := v.currentImageSortMode(first); got != imageSortByCreationDateDescending {
		t.Fatalf("first directory mode = %v, want creation date", got)
	}
	if got := v.currentImageSortMode(second); got != imageSortByNameAscending {
		t.Fatalf("unsaved directory mode = %v, want name", got)
	}
}

func TestImageSortCycleContainsEveryDirection(t *testing.T) {
	want := []imageSortMode{
		imageSortByNameAscending,
		imageSortByNameDescending,
		imageSortByModificationDateAscending,
		imageSortByModificationDateDescending,
		imageSortByCreationDateAscending,
		imageSortByCreationDateDescending,
		imageSortByCaptureDateAscending,
		imageSortByCaptureDateDescending,
	}
	if int(imageSortModeCount) != len(want) {
		t.Fatalf("sort mode count = %d, want %d", imageSortModeCount, len(want))
	}
	for index, mode := range want {
		if int(mode) != index {
			t.Fatalf("sort mode %v is at index %d, want %d", mode, int(mode), index)
		}
	}
}

func TestCaptureDateSort(t *testing.T) {
	early := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, mode := range []imageSortMode{imageSortByCaptureDateAscending, imageSortByCaptureDateDescending} {
		images := []navigationImage{{name: "missing"}, {name: "late", captureTime: early.Add(time.Hour)}, {name: "early", captureTime: early}, {name: "equal", captureTime: early}}
		sortNavigationImages(images, mode)
		want := "early,equal,late,missing"
		if mode == imageSortByCaptureDateDescending {
			want = "late,early,equal,missing"
		}
		var names []string
		for _, image := range images {
			names = append(names, image.name)
		}
		if got := strings.Join(names, ","); got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
		if imageSortModeFromConfig(mode.configValue()) != mode {
			t.Fatal("sort mode does not survive persistence")
		}
	}
}

// Simulates a slow directory without timing-dependent filesystem assumptions.
type blockedNavigationFS struct {
	fstest.MapFS
	entered chan struct{}
	release chan struct{}
}

func (f blockedNavigationFS) ReadDir(name string) ([]fs.DirEntry, error) {
	close(f.entered)
	<-f.release
	return f.MapFS.ReadDir(name)
}

func TestNavigationScanDoesNotBlockAndLatestRequestWins(t *testing.T) {
	files := fstest.MapFS{"album/a.jpg": {}, "album/b.jpg": {}}
	slow := blockedNavigationFS{MapFS: files, entered: make(chan struct{}), release: make(chan struct{})}
	defer close(slow.release)
	v := Viewer{navigationDirectory: "album", navigationImages: []string{"album/b.jpg", "album/a.jpg"}}
	v.startNavigationScan(slow, "album", imageSortByCreationDateAscending)
	defer v.cancelNavigationScan()
	select {
	case <-slow.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("scan did not start")
	}
	oldResults := v.navigationScanResults
	v.collectNavigationScan()
	if v.navigationImages[0] != "album/b.jpg" {
		t.Fatal("pending scan changed the current order")
	}

	v.startNavigationScan(files, "album", imageSortByNameAscending)
	select {
	case result := <-v.navigationScanResults:
		v.navigationScanResults <- result
	case <-time.After(5 * time.Second):
		t.Fatal("new scan waited for obsolete scan")
	}
	// An obsolete worker may already have published before cancellation.
	oldResults <- navigationScanResult{images: []string{"obsolete.jpg"}}
	v.collectNavigationScan()
	if want := []string{"album/a.jpg", "album/b.jpg"}; !reflect.DeepEqual(v.navigationImages, want) {
		t.Fatalf("images = %v, want %v", v.navigationImages, want)
	}
	v.collectNavigationScan()
	if v.navigationImages[0] == "obsolete.jpg" {
		t.Fatal("obsolete result was applied")
	}
}

func TestScanNavigationImagesDatesAndCancellation(t *testing.T) {
	early := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	files := fstest.MapFS{
		"album/a.jpg":        {ModTime: early.Add(time.Hour)},
		"album/b.jpg":        {ModTime: early},
		"album/notes.txt":    {},
		"album/nested/c.jpg": {},
	}
	images, err := scanNavigationImages(context.Background(), files, "album", imageSortByModificationDateAscending)
	if want := []string{"album/b.jpg", "album/a.jpg"}; err != nil || !reflect.DeepEqual(images, want) {
		t.Fatalf("images = %v, err = %v, want %v", images, err, want)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := scanNavigationImages(ctx, files, "album", imageSortByCreationDateAscending); err != context.Canceled {
		t.Fatalf("error = %v, want cancellation", err)
	}
}

func waitForNavigationScan(t *testing.T, v *Viewer) {
	t.Helper()
	if v.navigationScanResults == nil {
		t.Fatal("no pending navigation scan")
	}
	select {
	case result := <-v.navigationScanResults:
		if result.err != nil {
			t.Fatal(result.err)
		}
		v.navigationScanResults <- result
		v.collectNavigationScan()
	case <-time.After(5 * time.Second):
		t.Fatal("navigation scan did not complete")
	}
}
