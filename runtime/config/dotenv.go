package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// LoadDotenv loads KEY=VALUE pairs from path into the process environment.
// Variables already present in the environment are never overridden, so the
// real environment always wins over .env values. A missing file is not an
// error.
//
// Supported syntax: blank lines, '#' comment lines, an optional "export "
// prefix, and single- or double-quoted values. There is no variable expansion,
// inline-comment stripping, or multiline support; applications needing full
// dotenv semantics can load a dedicated library before calling Load.
func LoadDotenv(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for index, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, found := strings.Cut(line, "=")
		if !found {
			return fmt.Errorf("dotenv %s line %d: expected KEY=VALUE", path, index+1)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("dotenv %s line %d: empty key", path, index+1)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, unquote(strings.TrimSpace(value))); err != nil {
			return err
		}
	}
	return nil
}

func unquote(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
