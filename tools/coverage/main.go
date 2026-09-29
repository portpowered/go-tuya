// Command coverage reports statement coverage for non-generated library code.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
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

const (
	defaultCoveragePercent     = 80
	coverageRecordFieldCount   = 3
	filteredProfilePermissions = 0o600
	generatedHeaderScanLimit   = 2048
)

var (
	errCoverageModesConflict  = errors.New("coverage profile has conflicting modes")
	errInvalidCoverageRecord  = errors.New("invalid coverage record")
	errInvalidCoveragePath    = errors.New("invalid coverage location")
	errInvalidStatementCount  = errors.New("invalid statement count")
	errInvalidExecutionCount  = errors.New("invalid execution count")
	errCoverageBlock          = errors.New("coverage block")
	errCoverageModeMissing    = errors.New("coverage profile has no mode header")
	errCoverageStatementsZero = errors.New("coverage profile has no non-generated pkg statements")
	errCoverageBelowMinimum   = errors.New("coverage")
	errModuleDirectiveMissing = errors.New("module directive not found")
)

func main() {
	profile := flag.String("profile", "coverage.out", "Go coverage profile")
	minimum := flag.Float64("min", defaultCoveragePercent, "minimum combined statement coverage percent")
	percentOnly := flag.Bool("percent-only", false, "print only the combined percentage")
	filteredProfile := flag.String("filtered-profile", "", "write a profile without generated files")

	flag.Parse()

	err := report(*profile, *minimum, *percentOnly, *filteredProfile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func report(profile string, minimum float64, percentOnly bool, filteredProfile string) error {
	return reportInDirectory(".", profile, minimum, percentOnly, filteredProfile)
}

func reportInDirectory(root, profile string, minimum float64, percentOnly bool, filteredProfile string) (returnErr error) {
	file, module, err := openCoverageProfile(root, profile)
	if err != nil {
		return err
	}

	defer func() {
		closeErr := file.Close()
		if closeErr != nil && returnErr == nil {
			returnErr = fmt.Errorf("close coverage profile %q: %w", profile, closeErr)
		}
	}()

	scanner := bufio.NewScanner(file)

	mode, blocks, err := readCoverageBlocks(root, module, profile, scanner)
	if err != nil {
		return err
	}

	if mode == "" {
		return errCoverageModeMissing
	}

	filtered, byPackage := aggregateCoverage(mode, blocks)

	if filteredProfile != "" {
		err := os.WriteFile(filepath.Join(root, filteredProfile), []byte(filtered), filteredProfilePermissions)
		if err != nil {
			return fmt.Errorf("write filtered coverage profile %q: %w", filteredProfile, err)
		}
	}

	measured, err := printCoverageSummary(byPackage, percentOnly)
	if err != nil {
		return err
	}

	if measured+1e-9 < minimum {
		return fmt.Errorf("%w %.1f%% is below %.1f%% minimum", errCoverageBelowMinimum, measured, minimum)
	}

	return nil
}

func openCoverageProfile(root, profile string) (*os.File, string, error) {
	module, err := modulePath(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, "", err
	}

	// #nosec G304 -- -profile explicitly selects the coverage file to inspect.
	file, err := os.Open(filepath.Join(root, profile))
	if err != nil {
		return nil, "", fmt.Errorf("open coverage profile %q: %w", profile, err)
	}

	return file, module, nil
}

func aggregateCoverage(mode string, blocks map[string]coverageBlock) (string, map[string]totals) {
	byPackage := make(map[string]totals)

	keys := make([]string, 0, len(blocks))
	for location := range blocks {
		keys = append(keys, location)
	}

	sort.Strings(keys)

	var filtered strings.Builder

	filtered.WriteString(mode + "\n")

	for _, location := range keys {
		block := blocks[location]
		count := coveredCount(block.covered)
		fmt.Fprintf(&filtered, "%s %d %d\n", location, block.statements, count)
		packageName := path.Dir(block.source)
		byPackage[packageName] = addCoverage(byPackage[packageName], block)
	}

	return filtered.String(), byPackage
}

func coveredCount(covered bool) int {
	if covered {
		return 1
	}

	return 0
}

func addCoverage(total totals, block coverageBlock) totals {
	total.all += block.statements
	if block.covered {
		total.covered += block.statements
	}

	return total
}

func printCoverageSummary(byPackage map[string]totals, percentOnly bool) (float64, error) {
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
		return 0, errCoverageStatementsZero
	}

	measured := percent(combined)
	if percentOnly {
		fmt.Printf("%.1f\n", measured)
	} else {
		fmt.Printf("combined non-generated pkg coverage: %.1f%% (%d/%d statements)\n", measured, combined.covered, combined.all)
	}

	return measured, nil
}

func readCoverageBlocks(root, module, profile string, scanner *bufio.Scanner) (string, map[string]coverageBlock, error) {
	generated := make(map[string]bool)
	blocks := make(map[string]coverageBlock)
	mode := ""

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "mode:") {
			if mode != "" && mode != line {
				return "", nil, fmt.Errorf("%w: %q and %q", errCoverageModesConflict, mode, line)
			}

			mode = line

			continue
		}

		location, block, included, err := parseCoverageRecord(root, module, generated, line)
		if err != nil {
			return "", nil, err
		}

		if included {
			err = mergeCoverageBlock(blocks, location, block)
			if err != nil {
				return "", nil, err
			}
		}
	}

	scanErr := scanner.Err()
	if scanErr != nil {
		return "", nil, fmt.Errorf("scan coverage profile %q: %w", profile, scanErr)
	}

	return mode, blocks, nil
}

func parseCoverageRecord(root, module string, generated map[string]bool, line string) (string, coverageBlock, bool, error) {
	fields := strings.Fields(line)
	if len(fields) != coverageRecordFieldCount {
		return "", coverageBlock{}, false, fmt.Errorf("%w: %q", errInvalidCoverageRecord, line)
	}

	location := fields[0]

	colon := strings.LastIndexByte(location, ':')
	if colon < 0 {
		return "", coverageBlock{}, false, fmt.Errorf("%w: %q", errInvalidCoveragePath, location)
	}

	source, included, err := coverageSource(root, module, location[:colon], generated)
	if err != nil || !included {
		return location, coverageBlock{}, included, err
	}

	statements, count, err := parseCoverageCounts(fields, line)
	if err != nil {
		return "", coverageBlock{}, false, err
	}

	return location, coverageBlock{source: source, statements: statements, covered: count > 0}, true, nil
}

func coverageSource(root, module, location string, generated map[string]bool) (string, bool, error) {
	source := path.Clean(strings.TrimPrefix(strings.ReplaceAll(location, "\\", "/"), module+"/"))
	if path.IsAbs(source) || source == ".." || strings.HasPrefix(source, "../") {
		return "", false, fmt.Errorf("%w: %q escapes the package root", errInvalidCoveragePath, location)
	}

	if !strings.HasPrefix(source, "pkg/") || strings.HasPrefix(source, "pkg/testing/") {
		return source, false, nil
	}

	isGenerated, ok := generated[source]
	if !ok {
		// #nosec G304 -- source comes from a validated coverage record under pkg/.
		var err error

		isGenerated, err = generatedFile(filepath.Join(root, filepath.FromSlash(source)))
		if err != nil {
			return "", false, err
		}

		generated[source] = isGenerated
	}

	return source, !isGenerated, nil
}

func parseCoverageCounts(fields []string, line string) (int64, int64, error) {
	statements, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || statements < 0 {
		return 0, 0, fmt.Errorf("%w in %q", errInvalidStatementCount, line)
	}

	count, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || count < 0 {
		return 0, 0, fmt.Errorf("%w in %q", errInvalidExecutionCount, line)
	}

	return statements, count, nil
}

func mergeCoverageBlock(blocks map[string]coverageBlock, location string, block coverageBlock) error {
	previous, exists := blocks[location]
	if !exists {
		blocks[location] = block

		return nil
	}

	if previous.statements != block.statements {
		return fmt.Errorf("%w %q has conflicting statement counts", errCoverageBlock, location)
	}

	previous.covered = previous.covered || block.covered
	blocks[location] = previous

	return nil
}

func modulePath(filename string) (string, error) {
	// #nosec G304 -- filename is the go.mod path built from the selected report root.
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("read module file %q: %w", filename, err)
	}

	for line := range strings.SplitSeq(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[0] == "module" {
			return fields[1], nil
		}
	}

	return "", fmt.Errorf("%w in %s", errModuleDirectiveMissing, filename)
}

func generatedFile(filename string) (bool, error) {
	if strings.HasSuffix(filename, ".gen.go") {
		return true, nil
	}

	// #nosec G304 -- filename comes from a validated coverage record under pkg/.
	data, err := os.ReadFile(filename)
	if err != nil {
		return false, fmt.Errorf("read source file %q: %w", filename, err)
	}

	if len(data) > generatedHeaderScanLimit {
		data = data[:generatedHeaderScanLimit]
	}

	return strings.Contains(string(data), "Code generated") && strings.Contains(string(data), "DO NOT EDIT"), nil
}

func percent(value totals) float64 {
	return 100 * float64(value.covered) / float64(value.all)
}
