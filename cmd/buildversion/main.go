// Command buildversion exposes the project's canonical version resolver to all
// build and packaging scripts.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"viewergo/internal/buildversion"
)

func main() {
	explicit := flag.String("version", "", "explicit semantic version (optional v prefix)")
	format := flag.String("format", "lines", "output format: lines, display, windows, or json")
	flag.Parse()

	info, err := buildversion.Resolve(*explicit)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}

	switch *format {
	case "lines":
		fmt.Println(info.Version)
		fmt.Println(info.WindowsVersion)
	case "display":
		fmt.Println(info.Version)
	case "windows":
		fmt.Println(info.WindowsVersion)
	case "json":
		if err := json.NewEncoder(os.Stdout).Encode(info); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown format %q\n", *format)
		os.Exit(2)
	}
}
