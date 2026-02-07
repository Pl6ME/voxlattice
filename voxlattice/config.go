package voxlattice

const (
	defaultModel       = "models/gemini-2.5-flash-native-audio-preview-12-2025"
	defaultLogMaxBytes = 10 * 1024 * 1024
	sampleRateHz       = 24000
	channels           = 1
	bitsPerSample      = 16
	maxTextLen         = 10000
)

// Default voices (fallback when Voices.json missing or invalid)
var defaultSupportedVoices = map[string]string{
	"charon":    "Charon - Male voice",
	"kore":      "Kore - Female voice",
	"angus":     "Angus - Male voice",
	"brian":     "Brian - Male voice",
	"davis":     "Davis - Male voice",
	"emil":      "Emil - Male voice",
	"ethan":     "Ethan - Male voice",
	"greg":      "Greg - Male voice",
	"jeremy":    "Jeremy - Male voice",
	"joel":      "Joel - Male voice",
	"larry":     "Larry - Male voice",
	"paul":      "Paul - Male voice",
	"tim":       "Tim - Male voice",
	"will":      "Will - Male voice",
	"seraphina": "Seraphina - Female voice",
	"amber":     "Amber - Female voice",
	"emma":      "Emma - Female voice",
	"grace":     "Grace - Female voice",
	"ivy":       "Ivy - Female voice",
	"jessica":   "Jessica - Female voice",
	"karen":     "Karen - Female voice",
	"linda":     "Linda - Female voice",
	"olivia":    "Olivia - Female voice",
	"sarah":     "Sarah - Female voice",
	"violet":    "Violet - Female voice",
	"zoe":       "Zoe - Female voice",
	"alex":      "Alex - Neutral voice",
	"eric":      "Eric - Male voice",
	"jason":     "Jason - Male voice",
	"justin":    "Justin - Male voice",
}

// Supported voice names for Gemini TTS (loaded at startup)
var supportedVoices = map[string]string{}

type ttsReq struct {
	Text  string `json:"text"`
	Voice string `json:"voice,omitempty"`
	Lang  string `json:"lang,omitempty"` // Language code (e.g., "en-US", "zh-CN")
}

type healthResp struct {
	Status  string            `json:"status"`
	Model   string            `json:"model"`
	Voices  map[string]string `json:"voices"`
	Message string            `json:"message,omitempty"`
}

type errorResp struct {
	Error string `json:"error"`
}

type voiceItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type voicesEnvelope struct {
	GeneratedAt string      `json:"generated_at"`
	Voices      []voiceItem `json:"voices"`
}

type voicesMapEnvelope struct {
	GeneratedAt string            `json:"generated_at"`
	Voices      map[string]string `json:"voices"`
}
