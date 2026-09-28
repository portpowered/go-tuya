package main

import "testing"

func TestReleaseAllowsBreak(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		base        string
		release     string
		wantAllowed bool
	}{
		{name: "v0 minor permits break", base: "v0.1.9", release: "v0.2.0", wantAllowed: true},
		{name: "v0 patch rejects break", base: "v0.1.9", release: "v0.1.10"},
		{name: "stable minor rejects break", base: "v1.2.3", release: "v1.3.0"},
		{name: "major permits break", base: "v1.2.3", release: "v2.0.0", wantAllowed: true},
		{name: "v0 major permits break", base: "v0.9.9", release: "v1.0.0", wantAllowed: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			allowed, _, err := releaseAllowsBreak(test.base, test.release)
			if err != nil {
				t.Fatalf("releaseAllowsBreak() error = %v", err)
			}
			if allowed != test.wantAllowed {
				t.Errorf("releaseAllowsBreak() = %t, want %t", allowed, test.wantAllowed)
			}
		})
	}
}

func TestParsePackages(t *testing.T) {
	t.Parallel()

	got, err := parsePackages("tuya")
	if err != nil {
		t.Fatalf("parsePackages() error = %v", err)
	}
	if len(got) != 1 || got[0] != "tuya" {
		t.Errorf("parsePackages() = %#v, want the public tuya package", got)
	}
}

func TestFullPackagePath(t *testing.T) {
	t.Parallel()

	if got, want := fullPackagePath(defaultModulePath, "tuya"), "github.com/portpowered/go-tuya/tuya"; got != want {
		t.Errorf("fullPackagePath() = %q, want %q", got, want)
	}
}

func TestValidateReleaseVersionBeforeBaselineLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tag     string
		wantErr bool
	}{
		{tag: "v0.1.0"},
		{tag: "v1.2.3-rc.1"},
		{tag: "v01.2.3", wantErr: true},
		{tag: "v1.2", wantErr: true},
		{tag: "v1.2.3-01", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.tag, func(t *testing.T) {
			t.Parallel()

			if err := validateReleaseVersion(test.tag); (err != nil) != test.wantErr {
				t.Errorf("validateReleaseVersion(%q) error = %v, want error %t", test.tag, err, test.wantErr)
			}
		})
	}
}

func TestParsePackagesRejectsDuplicateOrEscapingPaths(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"tuya,tuya", "../private", "tuya,../private"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			if _, err := parsePackages(value); err == nil {
				t.Errorf("parsePackages(%q) error = nil, want error", value)
			}
		})
	}
}
