package api

import (
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/tiff"

	"github.com/navaneeth-ashok/c41invert/internal/conversion"
	"github.com/navaneeth-ashok/c41invert/internal/models"
)

// NotFoundHandler handles 404 errors with RFC 7807 format
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	// Only handle actual 404s, not root path if it has a specific handler
	if r.URL.Path != "/" {
		respondWithError(w, fmt.Sprintf("The requested endpoint '%s' does not exist", r.URL.Path), http.StatusNotFound)
		return
	}

	// Root path - return API info
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"name":    "C41 Invert API",
		"version": "1.0.0",
		"endpoints": []string{
			"GET  /health",
			"POST /convert",
		},
	}
	json.NewEncoder(w).Encode(response)
}

// HealthHandler is the API health check handler
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := models.HealthResponse{
		Status:  "healthy",
		Version: "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ConvertHandler is used for image conversion
func ConvertHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		respondWithError(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get uploaded file
	file, handler, err := r.FormFile("image")
	if err != nil {
		respondWithError(w, "No image file provided in 'image' field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Validate file extension - support RAW formats and common image formats
	ext := strings.ToLower(filepath.Ext(handler.Filename))
	supportedExts := []string{".tif", ".tiff", ".cr2", ".nef", ".raf", ".arw", ".dng", ".png", ".jpg", ".jpeg"}
	supported := false
	for _, validExt := range supportedExts {
		if ext == validExt {
			supported = true
			break
		}
	}
	if !supported {
		respondWithError(w,
			"Unsupported file format. Supported: TIFF, PNG, JPEG, and RAW formats (CR2, NEF, RAF, ARW, DNG)",
			http.StatusUnsupportedMediaType)
		return
	}

	// Parse conversion parameters
	params := parseConvertParams(r)

	// Create temp file
	tempFile, err := createTempFile(file, handler.Filename)
	if err != nil {
		respondWithError(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile)

	// Load and process image
	processedImage, err := processImage(tempFile, params)
	if err != nil {
		// Check if it's a decoding error (invalid TIFF)
		if strings.Contains(err.Error(), "tiff") || strings.Contains(err.Error(), "decode") {
			respondWithError(w, fmt.Sprintf("Invalid or corrupted TIFF file: %v", err), http.StatusBadRequest)
		} else {
			respondWithError(w, fmt.Sprintf("Failed to process image: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Validate output format
	outputFormat := strings.ToLower(params.OutputFormat)
	if outputFormat == "" {
		outputFormat = "jpeg" // default
	} else if outputFormat != "jpeg" && outputFormat != "tiff" {
		respondWithError(w, fmt.Sprintf("Unsupported output format '%s'. Must be 'jpeg' or 'tiff'", params.OutputFormat), http.StatusBadRequest)
		return
	}

	// Set response headers
	filename := fmt.Sprintf("converted_%d.%s", time.Now().Unix(), outputFormat)
	w.Header().Set("Content-Type", getContentType(outputFormat))
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

	// Encode and send processed image
	if outputFormat == "jpeg" {
		err = jpeg.Encode(w, processedImage, &jpeg.Options{Quality: 95})
	} else {
		err = tiff.Encode(w, processedImage, &tiff.Options{Compression: tiff.Deflate, Predictor: true})
	}

	if err != nil {
		// Can't send error response here since we already started writing the image
		// Log it instead
		fmt.Fprintf(os.Stderr, "Failed to encode output image: %v\n", err)
	}
}

func parseConvertParams(r *http.Request) models.ConvertRequest {
	params := models.ConvertRequest{
		SampleFraction: 0.8,  // defaults
		Lowlights:     0.01,
		Highlights:    0.99,
		OutputFormat:  "jpeg",
	}

	if val := r.FormValue("sample_fraction"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			params.SampleFraction = f
		}
	}

	if val := r.FormValue("lowlights"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			params.Lowlights = f
		}
	}

	if val := r.FormValue("highlights"); val != "" {
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			params.Highlights = f
		}
	}

	if val := r.FormValue("output_format"); val != "" {
		params.OutputFormat = val
	}

	params.SCurve = r.FormValue("s_curve") == "true"
	params.CenterMetering = r.FormValue("center_weighted_metering") == "true"

	return params
}

func createTempFile(src io.Reader, originalFilename string) (string, error) {
	ext := filepath.Ext(originalFilename)
	tempFile, err := os.CreateTemp("", "c41invert_*"+ext)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, src)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

func processImage(filename string, params models.ConvertRequest) (image.Image, error) {
	// Load image (using existing load logic)
	picture, err := conversion.LoadImage(filename)
	if err != nil {
		return nil, err
	}

	// Sample palette
	sampleArea := conversion.SampleBounds(params.SampleFraction, picture, params.CenterMetering)
	palette := conversion.SamplePalette(picture, sampleArea)

	// Create transformation
	t := conversion.Transformation{
		Red:      conversion.Range{Low: palette.Red.Percentile(params.Lowlights), High: palette.Red.Percentile(params.Highlights)},
		Green:    conversion.Range{Low: palette.Green.Percentile(params.Lowlights), High: palette.Green.Percentile(params.Highlights)},
		Blue:     conversion.Range{Low: palette.Blue.Percentile(params.Lowlights), High: palette.Blue.Percentile(params.Highlights)},
		Contrast: params.Lowlights - params.Highlights,
	}

	// Apply mapping
	var mapping conversion.Mapping
	if params.SCurve {
		mapping = t.Sigmoid()
	} else {
		mapping = t.Linear()
	}

	return mapping.Apply(picture), nil
}

func getContentType(format string) string {
	switch format {
	case "jpeg":
		return "image/jpeg"
	case "tiff":
		return "image/tiff"
	default:
		return "application/octet-stream"
	}
}

func respondWithError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(code)

	problem := models.ProblemDetail{
		Type:   "about:blank",
		Title:  http.StatusText(code),
		Status: code,
		Detail: message,
	}
	json.NewEncoder(w).Encode(problem)
}