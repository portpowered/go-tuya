// Command modulepath verifies a nested Go module's declared import path.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	directory := flag.String("dir", "", "directory containing go.mod")
	wantedPath := flag.String("want", "", "required module import path")

	flag.Parse()

	if *directory == "" || *wantedPath == "" {
		fatalf("both -dir and -want are required")
	}

	contents, err := os.ReadFile(filepath.Join(*directory, "go.mod"))
	if err != nil {
		fatalf("read module file: %v", err)
	}

	for line := range strings.SplitSeq(string(contents), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "module" {
			continue
		}

		if len(fields) == 2 && fields[1] == *wantedPath {
			return
		}

		fatalf("module path is %q, want %q", strings.Join(fields[1:], " "), *wantedPath)
	}

	fatalf("module directive not found in %s", filepath.Join(*directory, "go.mod"))
}

func fatalf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)

	os.Exit(1)
}
