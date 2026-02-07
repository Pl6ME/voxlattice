package voxlattice

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"
)

func Run() {
	install := flag.Bool("install", false, "install as a system service")
	uninstall := flag.Bool("uninstall", false, "uninstall system service")
	serviceName := flag.String("service-name", "voxlattice", "service name")
	envPathFlag := flag.String("env", "", "path to .env file")
	configDirFlag := flag.String("config", ".", "config directory for Voices.json")
	logPathFlag := flag.String("log", "", "log file path")
	logLevelFlag := flag.String("log-level", "warn", "log level: debug|info|warn|error")
	flag.Parse()

	if *install && *uninstall {
		fmt.Fprintln(os.Stderr, "cannot use --install and --uninstall together")
		os.Exit(2)
	}

	level, err := parseLogLevel(*logLevelFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := initLogger(*logPathFlag, level); err != nil {
		fmt.Fprintln(os.Stderr, "init logger failed:", err)
		os.Exit(2)
	}

	if *install {
		envPath := resolveInstallEnvPath(*envPathFlag, *serviceName)
		args := buildServiceArgs(*logPathFlag, level.String(), *configDirFlag)
		if err := installService(*serviceName, envPath, args); err != nil {
			appLog.Fatalf("install failed: %v", err)
		}
		appLog.Infof("Service installed: %s", *serviceName)
		return
	}

	if *uninstall {
		envPath := resolveInstallEnvPath(*envPathFlag, *serviceName)
		if err := uninstallService(*serviceName, envPath); err != nil {
			appLog.Fatalf("uninstall failed: %v", err)
		}
		appLog.Infof("Service uninstalled: %s", *serviceName)
		return
	}

	envPath := resolveRunEnvPath(*envPathFlag, *configDirFlag)
	loadDotEnv(envPath)
	ensureEnv(envPath, map[string]string{
		"GEMINI_API_KEY": "your_api_key_here",
		"GEMINI_MODEL":   defaultModel,
		"AUDIOMESH_PORT": "8080",
	})
	voices, source, err := loadSupportedVoices(*configDirFlag)
	if err != nil {
		appLog.Fatalf("load voices failed: %v", err)
	}
	supportedVoices = voices
	appLog.Infof("Voices loaded from: %s", source)

	http.HandleFunc("/tts", ttsHandler)
	http.HandleFunc("/health", healthHandler)
	http.HandleFunc("/voices", voicesHandler)

	addr, err := getListenAddr()
	if err != nil {
		appLog.Fatalf("invalid port: %v", err)
	}
	fmt.Printf("Voxlattice starting on %s\n", addr)
	fmt.Printf("Voices loaded: %d\n", len(supportedVoices))
	appLog.Infof("TTS service starting on %s", addr)
	appLog.Infof("Supported voices: %v", getSupportedVoiceNames())
	server := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		appLog.Fatalf("server failed: %v", err)
	}
}
