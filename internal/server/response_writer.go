package server

import (
	"bytes"
	"net/http"

	"github.com/github/gh-aw-mcpg/internal/logger"
)

var logResponseWriter = logger.New("server:response_writer")

// responseWriter wraps http.ResponseWriter to capture response body and status code
// This unified implementation replaces loggingResponseWriter and sdkLoggingResponseWriter
type responseWriter struct {
	http.ResponseWriter
	body       bytes.Buffer
	statusCode int
}

// newResponseWriter creates a new responseWriter with default status code
func newResponseWriter(w http.ResponseWriter) *responseWriter {
	logResponseWriter.Print("Creating new response writer with default status 200")
	return &responseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
	}
}

func (w *responseWriter) WriteHeader(statusCode int) {
	logResponseWriter.Printf("Setting response status code: %d", statusCode)
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	logResponseWriter.Printf("Writing response body: %d bytes", len(b))
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// Body returns the captured response body as bytes
func (w *responseWriter) Body() []byte {
	bodyBytes := w.body.Bytes()
	logResponseWriter.Printf("Retrieving captured body: %d bytes", len(bodyBytes))
	return bodyBytes
}

// StatusCode returns the captured HTTP status code
func (w *responseWriter) StatusCode() int {
	logResponseWriter.Printf("Retrieving captured status code: %d", w.statusCode)
	return w.statusCode
}
