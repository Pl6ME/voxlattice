package voxlattice

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Voices list endpoint
func voicesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	names := getSupportedVoiceNames()
	list := make([]voiceItem, 0, len(names))
	for _, name := range names {
		list = append(list, voiceItem{
			Name:        name,
			Description: supportedVoices[name],
		})
	}
	json.NewEncoder(w).Encode(list)
}

// Helper function to get supported voice names
func getSupportedVoiceNames() []string {
	voices := supportedVoices
	if voices == nil {
		return nil
	}
	names := make([]string, 0, len(voices))
	for name := range voices {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func voicesFilePath(configDir string) string {
	dir := strings.TrimSpace(configDir)
	if dir == "" || dir == "." {
		return "Voices.json"
	}
	return filepath.Join(dir, "Voices.json")
}

func normalizeVoiceMap(in map[string]string) (map[string]string, error) {
	if len(in) == 0 {
		return nil, errors.New("voices list is empty")
	}
	out := make(map[string]string, len(in))
	for name, desc := range in {
		n := strings.ToLower(strings.TrimSpace(name))
		d := strings.TrimSpace(desc)
		if n == "" || d == "" {
			return nil, fmt.Errorf("invalid voice entry: name=%q desc=%q", name, desc)
		}
		out[n] = d
	}
	if len(out) == 0 {
		return nil, errors.New("voices list is empty after normalization")
	}
	return out, nil
}

func normalizeVoiceItems(items []voiceItem) (map[string]string, error) {
	if len(items) == 0 {
		return nil, errors.New("voices list is empty")
	}
	out := make(map[string]string, len(items))
	for _, item := range items {
		n := strings.ToLower(strings.TrimSpace(item.Name))
		d := strings.TrimSpace(item.Description)
		if n == "" || d == "" {
			return nil, fmt.Errorf("invalid voice item: name=%q desc=%q", item.Name, item.Description)
		}
		out[n] = d
	}
	if len(out) == 0 {
		return nil, errors.New("voices list is empty after normalization")
	}
	return out, nil
}

func parseVoicesJSONWithTimestamp(data []byte) (map[string]string, time.Time, bool, error) {
	var env voicesEnvelope
	if err := json.Unmarshal(data, &env); err == nil && len(env.Voices) > 0 {
		voices, err := normalizeVoiceItems(env.Voices)
		if err != nil {
			return nil, time.Time{}, false, err
		}
		ts, hasTS, err := parseGeneratedAt(env.GeneratedAt)
		if err != nil {
			return nil, time.Time{}, false, err
		}
		return voices, ts, hasTS, nil
	}
	var envMap voicesMapEnvelope
	if err := json.Unmarshal(data, &envMap); err == nil && len(envMap.Voices) > 0 {
		voices, err := normalizeVoiceMap(envMap.Voices)
		if err != nil {
			return nil, time.Time{}, false, err
		}
		ts, hasTS, err := parseGeneratedAt(envMap.GeneratedAt)
		if err != nil {
			return nil, time.Time{}, false, err
		}
		return voices, ts, hasTS, nil
	}
	var items []voiceItem
	if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
		voices, err := normalizeVoiceItems(items)
		return voices, time.Time{}, false, err
	}
	var voiceMap map[string]string
	if err := json.Unmarshal(data, &voiceMap); err == nil && len(voiceMap) > 0 {
		voices, err := normalizeVoiceMap(voiceMap)
		return voices, time.Time{}, false, err
	}
	var wrapper struct {
		Voices map[string]string `json:"voices"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Voices) > 0 {
		voices, err := normalizeVoiceMap(wrapper.Voices)
		return voices, time.Time{}, false, err
	}
	return nil, time.Time{}, false, errors.New("invalid voices json")
}

func parseGeneratedAt(value string) (time.Time, bool, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, false, nil
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("invalid generated_at: %w", err)
	}
	return ts, true, nil
}

func loadVoicesFromFile(path string) (map[string]string, time.Time, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, time.Time{}, false, err
	}
	return parseVoicesJSONWithTimestamp(data)
}

func writeVoicesFile(path string, voices map[string]string) error {
	if len(voices) == 0 {
		return errors.New("voices list is empty")
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(voices))
	for name := range voices {
		names = append(names, name)
	}
	sort.Strings(names)
	list := make([]voiceItem, 0, len(names))
	for _, name := range names {
		list = append(list, voiceItem{
			Name:        name,
			Description: voices[name],
		})
	}
	file := voicesEnvelope{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Voices:      list,
	}
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}

func loadSupportedVoices(configDir string) (map[string]string, string, error) {
	voicesPath := voicesFilePath(configDir)
	voices, _, _, err := loadVoicesFromFile(voicesPath)
	if err == nil {
		return voices, "file", nil
	}
	if !os.IsNotExist(err) {
		appLog.Warnf("Voices.json invalid: %v", err)
	}
	if len(defaultSupportedVoices) == 0 {
		return nil, "", fmt.Errorf("no default voices available and Voices.json missing or invalid: %s", voicesPath)
	}
	voices, err = normalizeVoiceMap(defaultSupportedVoices)
	if err != nil {
		return nil, "", err
	}
	if err := writeVoicesFile(voicesPath, voices); err != nil {
		appLog.Warnf("write Voices.json failed: %v", err)
	}
	return voices, "default", nil
}
