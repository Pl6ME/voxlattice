package voxlattice

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func isTempExecutable(path string) bool {
	tempDir := strings.ToLower(filepath.Clean(os.TempDir()))
	p := strings.ToLower(filepath.Clean(path))
	if strings.HasPrefix(p, tempDir) {
		return true
	}
	return strings.Contains(p, "go-build")
}

func quoteSystemdArg(value string) string {
	if value == "" {
		return "\"\""
	}
	if !strings.ContainsAny(value, " \t\"\\") {
		return value
	}
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch == '"' || ch == '\\' {
			b.WriteByte('\\')
		}
		b.WriteByte(ch)
	}
	b.WriteByte('"')
	return b.String()
}

func buildSystemdExecStart(execPath string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, quoteSystemdArg(execPath))
	for _, arg := range args {
		parts = append(parts, quoteSystemdArg(arg))
	}
	return strings.Join(parts, " ")
}

func quoteWindowsArg(value string) string {
	if value == "" {
		return "\"\""
	}
	if !strings.ContainsAny(value, " \t\"") {
		return value
	}
	var b strings.Builder
	b.WriteByte('"')
	backslashes := 0
	for i := 0; i < len(value); i++ {
		ch := value[i]
		if ch == '\\' {
			backslashes++
			continue
		}
		if ch == '"' {
			b.WriteString(strings.Repeat("\\", backslashes*2+1))
			b.WriteByte('"')
			backslashes = 0
			continue
		}
		if backslashes > 0 {
			b.WriteString(strings.Repeat("\\", backslashes))
			backslashes = 0
		}
		b.WriteByte(ch)
	}
	if backslashes > 0 {
		b.WriteString(strings.Repeat("\\", backslashes*2))
	}
	b.WriteByte('"')
	return b.String()
}

func buildWindowsBinPath(execPath string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	parts = append(parts, quoteWindowsArg(execPath))
	for _, arg := range args {
		parts = append(parts, quoteWindowsArg(arg))
	}
	return "binPath= " + strings.Join(parts, " ")
}

func buildSystemdService(execPath, workingDir, envPath string, args []string) string {
	var b strings.Builder
	b.WriteString("[Unit]\n")
	b.WriteString("Description=Voxlattice TTS Service\n")
	b.WriteString("After=network.target\n\n")
	b.WriteString("[Service]\n")
	b.WriteString("Type=simple\n")
	b.WriteString("ExecStart=")
	b.WriteString(buildSystemdExecStart(execPath, args))
	b.WriteString("\n")
	if workingDir != "" {
		b.WriteString("WorkingDirectory=")
		b.WriteString(workingDir)
		b.WriteString("\n")
	}
	if envPath != "" {
		b.WriteString("EnvironmentFile=")
		b.WriteString(envPath)
		b.WriteString("\n")
		b.WriteString("Environment=AUDIOMESH_ENV=")
		b.WriteString(envPath)
		b.WriteString("\n")
	}
	b.WriteString("Restart=on-failure\n")
	b.WriteString("RestartSec=2\n\n")
	b.WriteString("[Install]\n")
	b.WriteString("WantedBy=multi-user.target\n")
	return b.String()
}

func buildServiceArgs(logPath, logLevel, configDir string) []string {
	args := []string{}
	if logPath != "" {
		args = append(args, "--log", logPath)
	}
	if logLevel != "" {
		args = append(args, "--log-level", logLevel)
	}
	if configDir != "" && configDir != "." {
		args = append(args, "--config", configDir)
	}
	return args
}

func installLinuxService(serviceName, envPath string, args []string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable failed: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable symlink failed: %w", err)
	}
	if isTempExecutable(execPath) {
		return fmt.Errorf("refuse to install from temp executable: %s (please build and run the binary)", execPath)
	}
	workingDir := filepath.Dir(execPath)
	if envPath == "" {
		envPath = filepath.Join("/etc", serviceName+".env")
	}
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return fmt.Errorf("create env dir failed: %w", err)
	}
	ensureEnv(envPath, map[string]string{
		"GEMINI_API_KEY": "your_api_key_here",
		"GEMINI_MODEL":   defaultModel,
		"AUDIOMESH_PORT": "8080",
	})
	servicePath := filepath.Join("/etc/systemd/system", serviceName+".service")
	serviceBody := buildSystemdService(execPath, workingDir, envPath, args)
	if err := os.WriteFile(servicePath, []byte(serviceBody), 0644); err != nil {
		return fmt.Errorf("write service file failed: %w", err)
	}
	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %w", err)
	}
	if err := runCommand("systemctl", "enable", "--now", serviceName); err != nil {
		return fmt.Errorf("systemctl enable --now failed: %w", err)
	}
	return nil
}

func installWindowsService(serviceName, envPath string, args []string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable failed: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable symlink failed: %w", err)
	}
	if isTempExecutable(execPath) {
		return fmt.Errorf("refuse to install from temp executable: %s (please build and run the binary)", execPath)
	}
	if envPath == "" {
		envPath = filepath.Join(filepath.Dir(execPath), ".env")
	}
	if err := os.MkdirAll(filepath.Dir(envPath), 0755); err != nil {
		return fmt.Errorf("create env dir failed: %w", err)
	}
	ensureEnv(envPath, map[string]string{
		"GEMINI_API_KEY": "your_api_key_here",
		"GEMINI_MODEL":   defaultModel,
		"AUDIOMESH_PORT": "8080",
	})
	if err := runCommand("setx", "/M", "AUDIOMESH_ENV", envPath); err != nil {
		return fmt.Errorf("setx AUDIOMESH_ENV failed: %w", err)
	}
	binPath := buildWindowsBinPath(execPath, args)
	start := "start= auto"
	display := "DisplayName= Voxlattice"
	if err := runCommand("sc.exe", "create", serviceName, binPath, display, start); err != nil {
		return fmt.Errorf("sc create failed: %w", err)
	}
	_ = runCommand("sc.exe", "description", serviceName, "Voxlattice TTS Service")
	if err := runCommand("sc.exe", "start", serviceName); err != nil {
		return fmt.Errorf("sc start failed: %w", err)
	}
	return nil
}

func uninstallLinuxService(serviceName, envPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable failed: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable symlink failed: %w", err)
	}
	if isTempExecutable(execPath) {
		return fmt.Errorf("refuse to uninstall from temp executable: %s (please build and run the binary)", execPath)
	}
	var errs []string
	if err := runCommand("systemctl", "disable", "--now", serviceName); err != nil {
		errs = append(errs, "systemctl disable --now failed: "+err.Error())
	}
	servicePath := filepath.Join("/etc/systemd/system", serviceName+".service")
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		errs = append(errs, "remove service file failed: "+err.Error())
	}
	if err := runCommand("systemctl", "daemon-reload"); err != nil {
		errs = append(errs, "systemctl daemon-reload failed: "+err.Error())
	}
	if envPath != "" {
		if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, "remove env file failed: "+err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func uninstallWindowsService(serviceName, envPath string) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable failed: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("resolve executable symlink failed: %w", err)
	}
	if isTempExecutable(execPath) {
		return fmt.Errorf("refuse to uninstall from temp executable: %s (please build and run the binary)", execPath)
	}
	var errs []string
	if err := runCommand("sc.exe", "stop", serviceName); err != nil {
		errs = append(errs, "sc stop failed: "+err.Error())
	}
	if err := runCommand("sc.exe", "delete", serviceName); err != nil {
		errs = append(errs, "sc delete failed: "+err.Error())
	}
	if envPath != "" {
		if err := os.Remove(envPath); err != nil && !os.IsNotExist(err) {
			errs = append(errs, "remove env file failed: "+err.Error())
		}
		if v := os.Getenv("AUDIOMESH_ENV"); v != "" && v == envPath {
			_ = runCommand("setx", "/M", "AUDIOMESH_ENV", "")
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func installService(serviceName, envPath string, args []string) error {
	if runtime.GOOS == "windows" {
		return installWindowsService(serviceName, envPath, args)
	}
	return installLinuxService(serviceName, envPath, args)
}

func uninstallService(serviceName, envPath string) error {
	if runtime.GOOS == "windows" {
		return uninstallWindowsService(serviceName, envPath)
	}
	return uninstallLinuxService(serviceName, envPath)
}
