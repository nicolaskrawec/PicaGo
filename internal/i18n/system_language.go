package i18n

import (
	"os"
	"strings"
)

// languageFromEnvironment handles the locale format commonly exposed on
// Unix-like systems, for example fr_FR.UTF-8 or en_US@calendar.
func languageFromEnvironment() string {
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		language := strings.TrimSpace(os.Getenv(name))
		if language == "" || language == "C" || language == "POSIX" {
			continue
		}
		if index := strings.IndexAny(language, ".@"); index >= 0 {
			language = language[:index]
		}
		return strings.ReplaceAll(language, "_", "-")
	}
	return ""
}
