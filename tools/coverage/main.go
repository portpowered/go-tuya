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
	var filtered strings.Builder
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			filtered.WriteString(line + "\n")
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
		filtered.WriteString(line + "\n")
		statements, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil || statements < 0 {
			return fmt.Errorf("invalid statement count in %q", line)
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil || count < 0 {
			return fmt.Errorf("invalid execution count in %q", line)
		}
		packageName := path.Dir(source)
		entry := byPackage[packageName]
		entry.all += statements
		if count > 0 {
			entry.covered += statements
		}
		byPackage[packageName] = entry
	}
	if err := scanner.Err(); err != nil {
		return err
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
