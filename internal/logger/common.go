package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Close Pattern for Logger Types
//
// All logger types in this package should implement their Close() method using this pattern:
//
//	func (l *Logger) Close() error {
//	    l.mu.Lock()
//	    defer l.mu.Unlock()
//
//	    // Optional: Perform cleanup before closing (e.g., write footer)
//	    // if l.logFile != nil {
//	    //     if err := writeCleanup(); err != nil {
//	    //         return closeLogFile(l.logFile, &l.mu, "loggerName")
//	    //     }
//	    // }
//
//	    return closeLogFile(l.logFile, &l.mu, "loggerName")
//	}
//
// Why this pattern?
//
//  1. Mutex protection: Acquire lock at method entry to ensure thread-safe cleanup
//  2. Deferred unlock: Use defer to release lock even if errors occur
//  3. Optional cleanup: Logger-specific cleanup (like MarkdownLogger's footer) goes before closeLogFile
//  4. Shared helper: Always delegate to closeLogFile() for consistent sync and close behavior
//  5. Error handling: Return errors from closeLogFile to indicate serious issues
//
// Examples:
//
// Simple Close() with no cleanup (FileLogger, JSONLLogger):
//
//	func (fl *FileLogger) Close() error {
//	    fl.mu.Lock()
//	    defer fl.mu.Unlock()
//	    return closeLogFile(fl.logFile, &fl.mu, "file")
//	}
//
// Close() with custom cleanup (MarkdownLogger):
//
//	func (ml *MarkdownLogger) Close() error {
//	    ml.mu.Lock()
//	    defer ml.mu.Unlock()
//
//	    if ml.logFile != nil {
//	        // Write closing details tag before closing
//	        footer := "\n</details>\n"
//	        if _, err := ml.logFile.WriteString(footer); err != nil {
//	            // Even if footer write fails, try to close the file properly
//	            return closeLogFile(ml.logFile, &ml.mu, "markdown")
//	        }
//
//	        // Footer written successfully, now close
//	        return closeLogFile(ml.logFile, &ml.mu, "markdown")
//	    }
//	    return nil
//	}
//
// This pattern is intentionally duplicated across logger types rather than abstracted:
//   - It's a standard Go idiom for wrapper methods
//   - The duplication is minimal (5-14 lines per type)
//   - Each logger can customize cleanup as needed
//   - The shared closeLogFile() helper eliminates complex logic duplication
//
// When adding a new logger type, follow this pattern to ensure consistent behavior.

// Initialization Pattern for Logger Types
//
// The logger package follows a consistent initialization pattern across all logger types,
// with support for customizable fallback behavior. This section documents the pattern
// and explains the different fallback strategies used by each logger type.
//
// Standard Initialization Pattern:
//
// All logger types use the initLogger() generic helper function for initialization:
//
//	func Init*Logger(logDir, fileName string) error {
//	    logger, err := initLogger(
//	        logDir, fileName, fileFlags,
//	        setupFunc,    // Configure logger after file is opened
//	        errorHandler, // Handle initialization failures
//	    )
//	    initGlobal*Logger(logger)
//	    return err
//	}
//
// The initLogger() helper:
//  1. Attempts to create the log directory (if needed)
//  2. Opens the log file with specified flags (os.O_APPEND, os.O_TRUNC, etc.)
//  3. Calls setupFunc to configure the logger instance
//  4. On error, calls errorHandler to implement fallback behavior
//  5. Returns the initialized logger and any error
//
// Fallback Behavior Strategies:
//
// Different logger types implement different fallback strategies based on their purpose:
//
// 1. FileLogger - Stdout Fallback:
//    - Purpose: Operational logs must always be visible
//    - Fallback: Redirects to stdout if log directory/file creation fails
//    - Error: Returns nil (never fails, always provides output)
//    - Use case: Critical operational messages that must be seen
//
//    Example error handler:
//    func(err error, logDir, fileName string) (*FileLogger, error) {
//        log.Printf("WARNING: Failed to initialize log file: %v", err)
//        log.Printf("WARNING: Falling back to stdout for logging")
//        return &FileLogger{
//            logDir:      logDir,
//            fileName:    fileName,
//            useFallback: true,
//            logger:      log.New(os.Stdout, "", 0),
//        }, nil
//    }
//
// 2. MarkdownLogger - Silent Fallback:
//    - Purpose: GitHub workflow preview logs (optional enhancement)
//    - Fallback: Sets useFallback flag, produces no output
//    - Error: Returns nil (never fails, silently disables)
//    - Use case: Nice-to-have logs that shouldn't block operations
//
//    Example error handler:
//    func(err error, logDir, fileName string) (*MarkdownLogger, error) {
//        return &MarkdownLogger{
//            logDir:      logDir,
//            fileName:    fileName,
//            useFallback: true,
//        }, nil
//    }
//
// 3. JSONLLogger - Strict Mode:
//    - Purpose: Machine-readable RPC message logs for analysis
//    - Fallback: None - returns error immediately
//    - Error: Returns error to caller
//    - Use case: Structured data that requires file storage
//
//    Example error handler:
//    func(err error, logDir, fileName string) (*JSONLLogger, error) {
//        return nil, err
//    }
//
// 4. ServerFileLogger - Unified Fallback:
//    - Purpose: Per-server log files for troubleshooting
//    - Fallback: Sets useFallback flag, logs to unified logger only
//    - Error: Returns nil (never fails, falls back to unified logging)
//    - Use case: Per-server logs are helpful but not required
//
//    Note: ServerFileLogger doesn't use initLogger() because it creates
//    files on-demand, but follows the same fallback philosophy.
//
// Global Logger Management:
//
// After initialization, all logger types register themselves as global loggers
// using the generic initGlobal*Logger() helpers from global_helpers.go:
//
//  - initGlobalFileLogger()
//  - initGlobalJSONLLogger()
//  - initGlobalMarkdownLogger()
//  - initGlobalServerFileLogger()
//
// These helpers ensure thread-safe initialization with proper cleanup of any
// existing logger instance.
//
// When to Use Each Logger Type:
//
// - FileLogger: Required operational logs (startup, errors, warnings)
// - MarkdownLogger: Optional GitHub workflow preview logs
// - JSONLLogger: Structured RPC message logs for tooling/analysis
// - ServerFileLogger: Per-server troubleshooting logs
//
// Adding a New Logger Type:
//
// When adding a new logger type:
//  1. Implement Close() method following the Close Pattern (above)
//  2. Add type to closableLogger constraint in global_helpers.go
//  3. Use initLogger() for initialization with appropriate fallback strategy
//  4. Add initGlobal*Logger() and closeGlobal*Logger() helpers
//  5. Document the fallback strategy and use case
//
// This consistent pattern ensures:
//  - Predictable behavior across all loggers
//  - Easy to understand fallback strategies
//  - Minimal code duplication
//  - Type-safe global logger management
//
// See file_logger.go, jsonl_logger.go, markdown_logger.go, and server_file_logger.go
// for complete implementation examples.

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
//   - onError: Function to handle initialization errors (implements fallback strategy)
//
// Returns:
//   - T: The initialized logger instance
//   - error: Any error that occurred during initialization
//
// This function:
//  1. Attempts to open the log file with the specified flags
//  2. If successful, calls the setup function to configure the logger
//  3. If unsuccessful, calls the error handler to implement the logger's fallback strategy
//
// The onError handler determines the fallback behavior. See "Initialization Pattern for Logger Types"
// documentation above for details on fallback strategies:
//   - FileLogger: Falls back to stdout
//   - MarkdownLogger: Silent fallback (no output)
//   - JSONLLogger: Returns error (no fallback)
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
