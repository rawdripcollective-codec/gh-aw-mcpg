package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// closeLogFile is a common helper for closing log files with consistent error handling.
// It syncs buffered data before closing and handles errors appropriately.
// The mutex should already be held by the caller.
//
// Error handling strategy:
// - Sync errors are logged but don't prevent closing (ensures resources are released)
// - Close errors are returned to the caller
//
// This ensures consistent behavior across all logger types:
// - Resources are always released (no file descriptor leaks)
// - Sync errors are logged for debugging but don't block cleanup
// - Close errors are propagated to indicate serious issues
func closeLogFile(file *os.File, mu *sync.Mutex, loggerName string) error {
	if file == nil {
		return nil
	}

	// Sync any remaining buffered data before closing
	// Log errors but continue with close to avoid resource leaks
	if err := file.Sync(); err != nil {
		log.Printf("WARNING: Failed to sync %s log file before close: %v", loggerName, err)
	}

	// Always close the file, even if sync failed
	return file.Close()
}

// initLogFile handles the common logic for initializing a log file.
// It creates the log directory if needed and opens the log file with the specified flags.
//
// Parameters:
//   - logDir: Directory where the log file should be created
//   - fileName: Name of the log file
//   - flags: File opening flags (e.g., os.O_APPEND, os.O_TRUNC)
//
// Returns:
//   - *os.File: The opened log file handle
//   - error: Any error that occurred during directory creation or file opening
//
// This function does not implement any fallback behavior - it returns errors to the caller.
// Callers can decide whether to fall back to stdout or propagate the error.
func initLogFile(logDir, fileName string, flags int) (*os.File, error) {
	// Try to create the log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Try to open the log file with the specified flags
	logPath := filepath.Join(logDir, fileName)
	file, err := os.OpenFile(logPath, flags|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return file, nil
}

// loggerSetupFunc is a function type that sets up a logger instance after the log file is opened.
// It receives the opened file, logDir, and fileName, and returns the configured logger.
type loggerSetupFunc[T closableLogger] func(file *os.File, logDir, fileName string) (T, error)

// loggerErrorHandlerFunc is a function type that handles errors during logger initialization.
// It receives the error and returns a configured logger (possibly a fallback) or an error.
type loggerErrorHandlerFunc[T closableLogger] func(err error, logDir, fileName string) (T, error)

// initLogger is a generic function that handles common logger initialization logic.
// It reduces code duplication across FileLogger, JSONLLogger, and MarkdownLogger initialization.
//
// Type parameters:
//   - T: Any type that satisfies the closableLogger constraint
//
// Parameters:
//   - logDir: Directory where the log file should be created
//   - fileName: Name of the log file
//   - flags: File opening flags (e.g., os.O_APPEND, os.O_TRUNC)
//   - setup: Function to configure the logger after the file is opened
//   - onError: Function to handle initialization errors (can return fallback or error)
//
// Returns:
//   - T: The initialized logger instance
//   - error: Any error that occurred during initialization
//
// This function:
//  1. Attempts to open the log file with the specified flags
//  2. If successful, calls the setup function to configure the logger
//  3. If unsuccessful, calls the error handler to decide on fallback behavior
func initLogger[T closableLogger](
	logDir, fileName string,
	flags int,
	setup loggerSetupFunc[T],
	onError loggerErrorHandlerFunc[T],
) (T, error) {
	file, err := initLogFile(logDir, fileName, flags)
	if err != nil {
		return onError(err, logDir, fileName)
	}

	logger, err := setup(file, logDir, fileName)
	if err != nil {
		// If setup fails, close the file and return the error
		file.Close()
		// Return zero value for T (nil for pointer types)
		var zero T
		return zero, err
	}

	return logger, nil
}
