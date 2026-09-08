package app

import (
	"path/filepath"
	"strings"
	"testing"
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
