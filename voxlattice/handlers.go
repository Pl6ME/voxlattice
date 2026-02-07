package voxlattice

import (
	"encoding/json"
	"net/http"
	"os"
)

// Health check endpoint
func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	response := healthResp{
		Status:  "healthy",
		Model:   getModelName(),
		Voices:  supportedVoices,
		Message: "Voxlattice TTS service ready with custom voice support",
	}

	json.NewEncoder(w).Encode(response)
}

func getModelName() string {
	modelName := os.Getenv("GEMINI_MODEL")
	if modelName == "" {
		modelName = defaultModel
	}
	return modelName
}
