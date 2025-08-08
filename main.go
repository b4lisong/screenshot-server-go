package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"log"
	"net/http"

	"github.com/b4lisong/screenshot-server-go/screenshot"
)

func main() {
	port := flag.Int("p", 8080, "port to run the server on")
	flag.Parse()

	// Call handleScreenshot() when someone visits /screenshot
	http.HandleFunc("/screenshot", handleScreenshot)

	log.Printf("🟢 Server started at http://localhost:%d", *port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
	if err != nil {
		log.Fatalf("❌ Server failed to start: %v", err)
	}
}

func handleScreenshot(w http.ResponseWriter, r *http.Request) {
	log.Printf("📸 Received screenshot request from %s", r.RemoteAddr)
	img, err := screenshot.Capture()
	if err != nil {
		log.Printf("❌ Capture failed: %v", err)
		http.Error(w, "Failed to capture screenshot", http.StatusInternalServerError)
		return
	}
	log.Printf("✅ Screenshot captured successfully for %s", r.RemoteAddr)

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
