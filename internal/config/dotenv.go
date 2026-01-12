package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadDotenv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		if _, ok := os.LookupEnv(key); ok {
			continue
		}
		val = strings.Trim(val, `"'`)
		_ = os.Setenv(key, val)
	}
}

func EnsureConfigs(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if err := ensureFromExample(dir, ".env"); err != nil {
		return err
	}
	return nil
}

func ensureFromExample(dir, name string) error {
	target := filepath.Join(dir, name)
	example := target + ".example"

	if _, err := os.Stat(target); err == nil {
		return nil
	}
	if _, err := os.Stat(example); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("missing example file: %s", example)
		}
		return err
	}

	data, err := os.ReadFile(example)
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0644)
}
