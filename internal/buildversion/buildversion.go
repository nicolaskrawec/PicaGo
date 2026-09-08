// Package buildversion resolves the application version from an explicit
// semantic version or from the nearest reachable Git tag.
package buildversion

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	semverPattern   = regexp.MustCompile(`^(?:v)?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-((?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	describePattern = regexp.MustCompile(`^v(.+)-([0-9]+)-g([0-9a-f]+)$`)
)

// Info contains the human-readable application version and the four-part
// numeric version required by Windows resources.
type Info struct {
	Version        string `json:"version"`
	WindowsVersion string `json:"windowsVersion"`
}

// Resolve validates an explicit version when provided. Otherwise it derives a
// development version from the closest reachable vX.Y.Z Git tag.
func Resolve(explicit string) (Info, error) {
	if explicit != "" {
		return FromExplicit(explicit)
	}

	describe, err := git("describe", "--tags", "--match", "v[0-9]*", "--long", "--abbrev=7")
	if err == nil {
		dirty := gitDirty()
		return FromDescribe(describe, dirty)
	}

	sha, shaErr := git("rev-parse", "--short=7", "HEAD")
	if shaErr != nil {
		return Info{Version: "0.0.0-dev." + time.Now().UTC().Format("20060102150405"), WindowsVersion: "0.0.0.0"}, nil
	}

	count := 0
	if value, countErr := git("rev-list", "HEAD", "--count"); countErr == nil {
		count, _ = strconv.Atoi(value)
	}
	version := fmt.Sprintf("0.0.0-%d-g%s", count, sha)
	if gitDirty() {
		version += "-dirty"
	}
	return Info{Version: version, WindowsVersion: numericVersion(0, 0, 0, count)}, nil
}

// FromExplicit validates and normalizes a semantic version. The optional v tag
// prefix is deliberately not part of the application version.
func FromExplicit(value string) (Info, error) {
	matches := semverPattern.FindStringSubmatch(value)
	if matches == nil {
		return Info{}, fmt.Errorf("invalid semantic version %q (expected vMAJOR.MINOR.PATCH or MAJOR.MINOR.PATCH)", value)
	}

	major := windowsPart(matches[1])
	minor := windowsPart(matches[2])
	patch := windowsPart(matches[3])
	normalized := strings.TrimPrefix(value, "v")
	return Info{Version: normalized, WindowsVersion: numericVersion(major, minor, patch, 0)}, nil
}

// FromDescribe converts output such as v1.2.0-70-gd960d93 into application and
// Windows versions.
func FromDescribe(value string, dirty bool) (Info, error) {
	matches := describePattern.FindStringSubmatch(value)
	if matches == nil {
		return Info{}, fmt.Errorf("unexpected git describe output %q", value)
	}

	base, err := FromExplicit(matches[1])
	if err != nil {
		return Info{}, fmt.Errorf("invalid version tag in %q: %w", value, err)
	}
	commits, _ := strconv.Atoi(matches[2])
	version := base.Version
	if commits > 0 {
		version = fmt.Sprintf("%s-%d-g%s", version, commits, matches[3])
	}
	if dirty {
		version += "-dirty"
	}
	return Info{Version: version, WindowsVersion: numericVersionPart(base.WindowsVersion, commits)}, nil
}

func numericVersionPart(base string, build int) string {
	parts := strings.Split(base, ".")
	return strings.Join(parts[:3], ".") + "." + strconv.Itoa(clampWindowsPart(build))
}

func numericVersion(major, minor, patch, build int) string {
	return fmt.Sprintf("%d.%d.%d.%d", clampWindowsPart(major), clampWindowsPart(minor), clampWindowsPart(patch), clampWindowsPart(build))
}

func clampWindowsPart(value int) int {
	if value < 0 {
		return 0
	}
	if value > 65535 {
		return 65535
	}
	return value
}

func windowsPart(value string) int {
	parsed, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 65535
	}
	return int(parsed)
}

func git(args ...string) (string, error) {
	output, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(output)), err
}

func gitDirty() bool {
	output, err := exec.Command("git", "status", "--porcelain", "--untracked-files=normal").Output()
	return err == nil && len(strings.TrimSpace(string(output))) > 0
}
