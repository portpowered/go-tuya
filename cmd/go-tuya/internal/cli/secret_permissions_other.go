//go:build !windows

package cli

func restrictSecretFile(string) error { return nil }
