package voxlattice

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func resolveRunEnvPath(flagVal, configDir string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("AUDIOMESH_ENV"); v != "" {
		return v
	}
	dir := strings.TrimSpace(configDir)
	if dir == "" || dir == "." {
		return ".env"
	}
	return filepath.Join(dir, ".env")
}

func resolveInstallEnvPath(flagVal, serviceName string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := os.Getenv("AUDIOMESH_ENV"); v != "" {
		return v
	}
	if runtime.GOOS == "windows" {
		exePath, err := os.Executable()
		if err != nil {
			return ".env"
		}
		return filepath.Join(filepath.Dir(exePath), ".env")
	}
	return filepath.Join("/etc", serviceName+".env")
}

func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)

		if key == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

func parseDotEnv(path string) map[string]string {
	out := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		val = strings.Trim(val, `"'`)

		if key != "" {
			out[key] = val
		}
	}
	return out
}

func writeDotEnv(path string, kv map[string]string) {
	var b strings.Builder
	for k, v := range kv {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString("\n")
	}
	_ = os.WriteFile(path, []byte(b.String()), 0644)
}

func ensureEnv(path string, defaults map[string]string) {
	existing := parseDotEnv(path)
	changed := false

	for k, v := range defaults {
		current, ok := existing[k]
		if !ok || strings.TrimSpace(current) == "" {
			if envVal, okEnv := os.LookupEnv(k); okEnv && strings.TrimSpace(envVal) != "" {
				existing[k] = envVal
			} else {
				existing[k] = v
			}
			changed = true
		}

		if envVal, okEnv := os.LookupEnv(k); !okEnv || strings.TrimSpace(envVal) == "" {
			_ = os.Setenv(k, existing[k])
		}
	}

	if changed {
		dir := filepath.Dir(path)
		if dir != "" && dir != "." {
			_ = os.MkdirAll(dir, 0755)
		}
		writeDotEnv(path, existing)
	}
}
