package voxlattice

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

func pcmToWav(pcm []byte) ([]byte, error) {
	byteRate := sampleRateHz * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataLen := uint32(len(pcm))
	riffLen := 36 + dataLen

	buf := &bytes.Buffer{}
	buf.WriteString("RIFF")
	if err := binary.Write(buf, binary.LittleEndian, uint32(riffLen)); err != nil {
		return nil, err
	}
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	if err := binary.Write(buf, binary.LittleEndian, uint32(16)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(1)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(channels)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(sampleRateHz)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint32(byteRate)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(blockAlign)); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample)); err != nil {
		return nil, err
	}
	buf.WriteString("data")
	if err := binary.Write(buf, binary.LittleEndian, dataLen); err != nil {
		return nil, err
	}
	buf.Write(pcm)

	return buf.Bytes(), nil
}

func getRequestAPIKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	if v := strings.TrimSpace(r.Header.Get("X-Gemini-Api-Key")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-API-Key")); v != "" {
		return v
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(auth) >= 7 && strings.EqualFold(auth[:7], "Bearer ") {
		return strings.TrimSpace(auth[7:])
	}
	return ""
}

func getListenAddr() (string, error) {
	port := strings.TrimSpace(os.Getenv("AUDIOMESH_PORT"))
	if port == "" {
		port = "8080"
	}
	for _, ch := range port {
		if ch < '0' || ch > '9' {
			return "", fmt.Errorf("invalid AUDIOMESH_PORT: %s", port)
		}
	}
	if port == "0" {
		return "", fmt.Errorf("invalid AUDIOMESH_PORT: %s", port)
	}
	return ":" + port, nil
}

func parseTTSRequest(r *http.Request) (ttsReq, error) {
	var out ttsReq
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return out, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return out, err
	}

	if v, ok := raw["voice"]; ok && len(v) > 0 {
		_ = json.Unmarshal(v, &out.Voice)
	}
	if v, ok := raw["lang"]; ok && len(v) > 0 {
		_ = json.Unmarshal(v, &out.Lang)
	}

	textRaw, ok := raw["text"]
	if !ok || len(textRaw) == 0 {
		return out, errors.New("text is required")
	}

	var text string
	if err := json.Unmarshal(textRaw, &text); err == nil {
		out.Text = text
		return out, nil
	}

	var arr []string
	if err := json.Unmarshal(textRaw, &arr); err == nil {
		out.Text = strings.Join(arr, "\n")
		return out, nil
	}

	var arrAny []interface{}
	if err := json.Unmarshal(textRaw, &arrAny); err == nil {
		parts := make([]string, 0, len(arrAny))
		for _, v := range arrAny {
			if s, ok := v.(string); ok {
				parts = append(parts, s)
			} else {
				return out, errors.New("text array must contain strings only")
			}
		}
		out.Text = strings.Join(parts, "\n")
		return out, nil
	}

	return out, errors.New("invalid text format")
}

func normalizeText(input string) (string, error) {
	s := strings.ReplaceAll(input, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("text is empty")
	}

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '\n' || r == '\t' {
			b.WriteRune(r)
			continue
		}
		if r < 32 || r == 127 {
			continue
		}
		b.WriteRune(r)
	}
	clean := b.String()
	if clean == "" {
		return "", errors.New("text is empty after cleaning")
	}

	lines := strings.Split(clean, "\n")
	var out []string
	emptyRun := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " \t")
		if strings.TrimSpace(line) == "" {
			emptyRun++
			if emptyRun > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		emptyRun = 0
		out = append(out, line)
	}
	result := strings.TrimSpace(strings.Join(out, "\n"))
	if result == "" {
		return "", errors.New("text is empty after normalization")
	}
	if len(result) > maxTextLen {
		return "", fmt.Errorf("text too long: %d > %d", len(result), maxTextLen)
	}
	return result, nil
}

func ttsHandler(w http.ResponseWriter, r *http.Request) {
	// Allow CORS for web access
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}

	req, err := parseTTSRequest(r)
	if err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	normalized, err := normalizeText(req.Text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Text = normalized

	// Validate voice if provided
	if req.Voice != "" {
		// Normalize voice name to lowercase
		req.Voice = strings.ToLower(req.Voice)
		if _, exists := supportedVoices[req.Voice]; !exists {
			http.Error(w, fmt.Sprintf("unsupported voice: %s, supported voices: %v", req.Voice, getSupportedVoiceNames()), http.StatusBadRequest)
			return
		}
	}

	apiKey := getRequestAPIKey(r)
	if apiKey == "" {
		apiKey = strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
		if apiKey == "" || apiKey == "your_api_key_here" {
			http.Error(w, "missing api key", http.StatusUnauthorized)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		http.Error(w, "client init failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	// Note: genai client typically manages connections itself, no explicit close needed

	modelName := getModelName()

	systemInstruction := &genai.Content{
		Parts: []*genai.Part{
			{Text: "You are a TTS engine. Repeat the user's text verbatim. Do not add, remove, translate, or rephrase. Output audio only."},
		},
	}

	cfg := &genai.LiveConnectConfig{
		ResponseModalities: []genai.Modality{genai.ModalityAudio},
		Temperature:        genai.Ptr[float32](0),
		SystemInstruction:  systemInstruction,
	}

	// Configure voice if specified
	if req.Voice != "" {
		cfg.SpeechConfig = &genai.SpeechConfig{
			VoiceConfig: &genai.VoiceConfig{
				PrebuiltVoiceConfig: &genai.PrebuiltVoiceConfig{
					VoiceName: req.Voice,
				},
			},
		}
	}

	// If language is specified, we can potentially add language-specific instructions
	if req.Lang != "" {
		// Add language-specific instruction to the system instruction
		langInstruction := fmt.Sprintf("Respond in %s language with appropriate pronunciation.", req.Lang)
		systemInstruction.Parts = append(systemInstruction.Parts, &genai.Part{Text: langInstruction})
	}

	session, err := client.Live.Connect(ctx, modelName, cfg)
	if err != nil {
		http.Error(w, "live connect failed: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer session.Close()

	turn := genai.NewContentFromText(req.Text, genai.RoleUser)
	err = session.SendClientContent(genai.LiveClientContentInput{
		Turns:        []*genai.Content{turn},
		TurnComplete: genai.Ptr(true),
	})
	if err != nil {
		http.Error(w, "clientContent send failed: "+err.Error(), http.StatusBadGateway)
		return
	}

	var pcm bytes.Buffer
	for {
		msg, err := session.Receive()
		if err != nil {
			http.Error(w, "read failed: "+err.Error(), http.StatusBadGateway)
			return
		}

		if msg.ServerContent != nil && msg.ServerContent.ModelTurn != nil {
			for _, p := range msg.ServerContent.ModelTurn.Parts {
				if p.InlineData != nil && len(p.InlineData.Data) > 0 {
					pcm.Write(p.InlineData.Data)
				}
			}
		}

		if msg.ServerContent != nil && (msg.ServerContent.TurnComplete || msg.ServerContent.GenerationComplete) {
			break
		}
	}

	// Convert to WAV format
	wav, err := pcmToWav(pcm.Bytes())
	if err != nil {
		http.Error(w, "wav encode failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(wav)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(wav)
}
