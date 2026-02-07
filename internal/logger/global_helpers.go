// Package logger provides structured logging for the MCP Gateway.
//
// This file contains generic helper functions for managing global logger state with proper
// mutex handling. These helpers encapsulate common patterns for initializing and
// closing global loggers (FileLogger, JSONLLogger, MarkdownLogger) to reduce code
// duplication while maintaining thread safety.
//
// Functions in this file follow a consistent pattern:
//
// - init*: Initialize a global logger with proper locking and cleanup of any existing logger
// - close*: Close and clear a global logger with proper locking
//
// These helpers are used internally by the logger package and should not be called
// directly by external code. Use the public Init* and Close* functions instead.
package logger

import "sync"

// closableLogger is a constraint for types that have a Close method.
// This is satisfied by *FileLogger, *JSONLLogger, *MarkdownLogger, and *ServerFileLogger.
type closableLogger interface {
	*FileLogger | *JSONLLogger | *MarkdownLogger | *ServerFileLogger
	Close() error
}

// initGlobalLogger is a generic helper that encapsulates the common pattern for
// initializing a global logger with proper mutex handling.
//
// Type parameters:
//   - T: Any pointer type that satisfies closableLogger constraint
//
// Parameters:
//   - mu: Mutex to protect access to the global logger
//   - current: Pointer to the current global logger instance
//   - newLogger: New logger instance to set as the global logger
//
// This function:
//  1. Acquires the mutex lock
//  2. Closes any existing logger if present
//  3. Sets the new logger as the global instance
//  4. Releases the mutex lock
func initGlobalLogger[T closableLogger](mu *sync.RWMutex, current *T, newLogger T) {
	mu.Lock()
	defer mu.Unlock()

	if *current != nil {
		(*current).Close()
	}
	*current = newLogger
}

// closeGlobalLogger is a generic helper that encapsulates the common pattern for
// closing and clearing a global logger with proper mutex handling.
//
// Type parameters:
//   - T: Any pointer type that satisfies closableLogger constraint
//
// Parameters:
//   - mu: Mutex to protect access to the global logger
//   - logger: Pointer to the global logger instance to close
//
// Returns:
//   - error: Any error returned by the logger's Close() method
//
// This function:
//  1. Acquires the mutex lock
//  2. Closes the logger if it exists
//  3. Sets the logger pointer to nil
//  4. Releases the mutex lock
//  5. Returns any error from the Close() operation
func closeGlobalLogger[T closableLogger](mu *sync.RWMutex, logger *T) error {
	mu.Lock()
	defer mu.Unlock()

	if *logger != nil {
		err := (*logger).Close()
		var zero T
		*logger = zero
		return err
	}
	return nil
}

// Type-specific helper functions that use the generic implementations above.
// These maintain backward compatibility with existing code.

// initGlobalFileLogger initializes the global FileLogger using the generic helper.
func initGlobalFileLogger(logger *FileLogger) {
	initGlobalLogger(&globalLoggerMu, &globalFileLogger, logger)
}

// closeGlobalFileLogger closes the global FileLogger using the generic helper.
func closeGlobalFileLogger() error {
	return closeGlobalLogger(&globalLoggerMu, &globalFileLogger)
}

// initGlobalJSONLLogger initializes the global JSONLLogger using the generic helper.
func initGlobalJSONLLogger(logger *JSONLLogger) {
	initGlobalLogger(&globalJSONLMu, &globalJSONLLogger, logger)
}

// closeGlobalJSONLLogger closes the global JSONLLogger using the generic helper.
func closeGlobalJSONLLogger() error {
	return closeGlobalLogger(&globalJSONLMu, &globalJSONLLogger)
}

// initGlobalMarkdownLogger initializes the global MarkdownLogger using the generic helper.
func initGlobalMarkdownLogger(logger *MarkdownLogger) {
	initGlobalLogger(&globalMarkdownMu, &globalMarkdownLogger, logger)
}

// closeGlobalMarkdownLogger closes the global MarkdownLogger using the generic helper.
func closeGlobalMarkdownLogger() error {
	return closeGlobalLogger(&globalMarkdownMu, &globalMarkdownLogger)
}

// initGlobalServerFileLogger initializes the global ServerFileLogger using the generic helper.
func initGlobalServerFileLogger(logger *ServerFileLogger) {
	initGlobalLogger(&globalServerLoggerMu, &globalServerFileLogger, logger)
}

// closeGlobalServerFileLogger closes the global ServerFileLogger using the generic helper.
func closeGlobalServerFileLogger() error {
	return closeGlobalLogger(&globalServerLoggerMu, &globalServerFileLogger)
}
