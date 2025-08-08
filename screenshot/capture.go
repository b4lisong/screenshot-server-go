// Package screenshot
package screenshot

import (
	"fmt"
	"image"

	"github.com/kbinani/screenshot"
)

// Capture returns an image of the primary display.
// Returns an error if capture fails or no display is found.
func Capture() (image.Image, error) {
	numDisplays := screenshot.NumActiveDisplays()
	if numDisplays == 0 {
		return nil, fmt.Errorf("No active displays found!")
	}

	// Get the bounding rectangle of the first display
	bounds := screenshot.GetDisplayBounds(0)

	// Capture the image within those bounds
	img, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, fmt.Errorf("Failed to capture screen: %w", err)
	}

	return img, nil
}
