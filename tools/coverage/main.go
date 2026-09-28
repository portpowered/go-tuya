// Command coverage reports statement coverage for non-generated library code.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
)

type totals struct {
	covered int64
	all     int64
}

type coverageBlock struct {
	source     string
	statements int64
	covered    bool
}

func main() {
	profile := flag.String("profile", "coverage.out", "Go coverage profile")
	minimum := flag.Float64("min", 80, "minimum combined statement coverage percent")
	percentOnly := flag.Bool("percent-only", false, "print only the combined percentage")
	filteredProfile := flag.String("filtered-profile", "", "write a profile without generated files")
	flag.Parse()
	if err := report(*profile, *minimum, *percentOnly, *filteredProfile); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func report(profile string, minimum float64, percentOnly bool, filteredProfile string) error {
	module, err := modulePath("go.mod")
	if err != nil {
		return err
	}
	file, err := os.Open(profile)
	if err != nil {
		return err
	}
	defer file.Close()

	byPackage := make(map[string]totals)
	generated := make(map[string]bool)
	blocks := make(map[string]coverageBlock)
	mode := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			if mode != "" && mode != line {
				return fmt.Errorf("coverage profile has conflicting modes: %q and %q", mode, line)
			}
			mode = line
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return fmt.Errorf("invalid coverage record: %q", line)
		}
		location := fields[0]
		colon := strings.LastIndexByte(location, ':')
		if colon < 0 {
			return fmt.Errorf("invalid coverage location: %q", location)
		}
		source := strings.TrimPrefix(strings.ReplaceAll(location[:colon], "\\", "/"), module+"/")
		if !strings.HasPrefix(source, "pkg/") || strings.HasPrefix(source, "pkg/testing/") {
			continue
		}
		isGenerated, ok := generated[source]
		if !ok {
			isGenerated, err = generatedFile(source)
			if err != nil {
				return err
			}
			generated[source] = isGenerated
		}
		if isGenerated {
			continue
		}
		statements, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || statements < 0 {
			return fmt.Errorf("invalid statement count in %q", line)
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || count < 0 {
			return fmt.Errorf("invalid execution count in %q", line)
		}
		if previous, exists := blocks[location]; exists {
			if previous.statements != statements {
				return fmt.Errorf("coverage block %q has conflicting statement counts", location)
			}
			previous.covered = previous.covered || count > 0
			blocks[location] = previous
		} else {
			blocks[location] = coverageBlock{source: source, statements: statements, covered: count > 0}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if mode == "" {
		return fmt.Errorf("coverage profile has no mode header")
	}
	keys := make([]string, 0, len(blocks))
	for location := range blocks {
		keys = append(keys, location)
	}
	sort.Strings(keys)
	var filtered strings.Builder
	filtered.WriteString(mode + "\n")
	for _, location := range keys {
		block := blocks[location]
		count := 0
		if block.covered {
			count = 1
		}
		fmt.Fprintf(&filtered, "%s %d %d\n", location, block.statements, count)
		packageName := path.Dir(block.source)
		entry := byPackage[packageName]
		entry.all += block.statements
		if block.covered {
			entry.covered += block.statements
		}
		byPackage[packageName] = entry
	}
	if filteredProfile != "" {
		if err := os.WriteFile(filteredProfile, []byte(filtered.String()), 0600); err != nil {
			return err
		}
	}
	packages := make([]string, 0, len(byPackage))
	for name := range byPackage {
		packages = append(packages, name)
	}
	sort.Strings(packages)
	var combined totals
	for _, name := range packages {
		entry := byPackage[name]
		combined.all += entry.all
		combined.covered += entry.covered
		if entry.all > 0 && !percentOnly {
			fmt.Printf("%s: %.1f%% (%d/%d statements)\n", name, percent(entry), entry.covered, entry.all)
		}
	}
	if combined.all == 0 {
		return fmt.Errorf("coverage profile has no non-generated pkg statements")
	}
	measured := percent(combined)
	if percentOnly {
		fmt.Printf("%.1f\n", measured)
	} else {
		fmt.Printf("combined non-generated pkg coverage: %.1f%% (%d/%d statements)\n", measured, combined.covered, combined.all)
	}
	if measured+1e-9 < minimum {
		return fmt.Errorf("coverage %.1f%% is below %.1f%% minimum", measured, minimum)
	}
	return nil
}

func modulePath(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}
	return "", fmt.Errorf("module directive not found in %s", filename)
}

func generatedFile(filename string) (bool, error) {
	if strings.HasSuffix(filename, ".gen.go") {
		return true, nil
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return false, err
	}
	if len(data) > 2048 {
		data = data[:2048]
	}
	return strings.Contains(string(data), "Code generated") && strings.Contains(string(data), "DO NOT EDIT"), nil
}

func percent(value totals) float64 {
	return 100 * float64(value.covered) / float64(value.all)
}
