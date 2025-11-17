package conversion

import (
	"image"
	"image/color"
	_ "image/png"
	"log"
	"os"
	"strings"
)

// LoadImage loads an image from file, supporting both regular formats and RAW
func LoadImage(filename string) (image.Image, error) {
	f, oerr := os.Open(filename)
	if oerr != nil {
		return nil, oerr
	}
	defer f.Close()

	log.Printf("Processing %s\n", filename)
	rawExtensions := []string{".cr2", ".nef", ".raf", ".arw", ".dng"}
	for _, ext := range rawExtensions {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			// Decode using golibraw
			img, err := ImportRaw(filename)
			if err != nil {
				return nil, err
			}
			return img, nil
		}
	}

	p, _, derr := image.Decode(f)
	if derr != nil {
		return nil, derr
	}

	return p, nil
}

// SamplePalette samples color palette from image region
func SamplePalette(picture image.Image, sampleArea image.Rectangle) *Palette {
	var palette Palette
	for x := sampleArea.Min.X; x < sampleArea.Max.X; x++ {
		for y := sampleArea.Min.Y; y < sampleArea.Max.Y; y++ {
			palette.Add(color.RGBA64Model.Convert(picture.At(x, y)).(color.RGBA64))
		}
	}
	return &palette
}

// SampleBounds gets a bounding box for a center fraction of the image
func SampleBounds(fraction float64, picture image.Image, centerMetering bool) image.Rectangle {
	bounds := picture.Bounds()
	width := bounds.Max.X - bounds.Min.X
	height := bounds.Max.Y - bounds.Min.Y

	// Check if center metering (square bounds) is requested
	if centerMetering {
		// Determine the shorter dimension
		minDim := width
		if height < width {
			minDim = height
		}

		// Calculate the size of the square sample area based on fraction
		squareSize := int(float64(minDim) * fraction)
		borderWidth := (width - squareSize) / 2
		borderHeight := (height - squareSize) / 2

		// Define the square sample area, centered within the original bounds
		return image.Rectangle{
			Min: image.Point{bounds.Min.X + borderWidth, bounds.Min.Y + borderHeight},
			Max: image.Point{bounds.Min.X + borderWidth + squareSize, bounds.Min.Y + borderHeight + squareSize},
		}
	}

	// Default behavior: aspect-ratio-preserving sample bounds
	border := (1 - fraction) / 2
	return image.Rectangle{
		Min: image.Point{
			X: bounds.Min.X + int(float64(width)*border),
			Y: bounds.Min.Y + int(float64(height)*border),
		},
		Max: image.Point{
			X: bounds.Max.X - int(float64(width)*border),
			Y: bounds.Max.Y - int(float64(height)*border),
		},
	}
}