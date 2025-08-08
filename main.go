package main

import (
	"bytes"
	"image/png"
	"log"
	"net/http"

	"github.com/b4lisong/screenshot-server-go/screenshot"
)

func main() {
	// Call handleScreenshot() when someone visits /screenshot
	http.HandleFunc("/screenshot", handleScreenshot)

	log.Println("🟢 Server started at http://localhost:8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}

func handleScreenshot(w http.ResponseWriter, r *http.Request) {
	img, err := screenshot.Capture()
	if err != nil {
		log.Printf("❌ Capture failed: %v", err)
		http.Error(w, "Failed to capture screenshot", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer // in-mem byte slice
	err = png.Encode(&buf, img)
	if err != nil {
		log.Printf("❌ Encoding failed: %v", err)
		http.Error(w, "Failed to encode image", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Content-Encoding", "identity")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(buf.Bytes())
	if err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
