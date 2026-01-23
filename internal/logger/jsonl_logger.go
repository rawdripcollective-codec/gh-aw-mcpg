package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/githubnext/gh-aw-mcpg/internal/logger/sanitize"
)

// JSONLLogger manages logging RPC messages to a JSONL file (one JSON object per line)
type JSONLLogger struct {
	logFile  *os.File
	mu       sync.Mutex
	logDir   string
	fileName string
	encoder  *json.Encoder
}

var (
	globalJSONLLogger *JSONLLogger
	globalJSONLMu     sync.RWMutex
)

// JSONLRPCMessage represents a single RPC message log entry in JSONL format
type JSONLRPCMessage struct {
	Timestamp string          `json:"timestamp"`
	Direction string          `json:"direction"` // "IN" or "OUT"
	Type      string          `json:"type"`      // "REQUEST" or "RESPONSE"
	ServerID  string          `json:"server_id"`
	Method    string          `json:"method,omitempty"`
	Error     string          `json:"error,omitempty"`
	Payload   json.RawMessage `json:"payload"` // Full sanitized payload as raw JSON
}

// InitJSONLLogger initializes the global JSONL logger
func InitJSONLLogger(logDir, fileName string) error {
	logger, err := initLogger(
		logDir, fileName, os.O_APPEND,
		// Setup function: configure the logger after file is opened
		func(file *os.File, logDir, fileName string) (*JSONLLogger, error) {
			jl := &JSONLLogger{
				logDir:   logDir,
				fileName: fileName,
				logFile:  file,
				encoder:  json.NewEncoder(file),
			}
			return jl, nil
		},
		// Error handler: return error immediately (no fallback)
		func(err error, logDir, fileName string) (*JSONLLogger, error) {
			return nil, err
		},
	)

	if err != nil {
		return err
	}

	initGlobalJSONLLogger(logger)
	return nil
}

// Close closes the JSONL log file
func (jl *JSONLLogger) Close() error {
	jl.mu.Lock()
	defer jl.mu.Unlock()

	return closeLogFile(jl.logFile, &jl.mu, "JSONL")
}

// LogMessage logs an RPC message to the JSONL file
func (jl *JSONLLogger) LogMessage(entry *JSONLRPCMessage) error {
	jl.mu.Lock()
	defer jl.mu.Unlock()

	if jl.logFile == nil {
		return fmt.Errorf("JSONL logger not initialized")
	}

	// Encode and write the JSON object followed by a newline
	if err := jl.encoder.Encode(entry); err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	// Flush to disk immediately
	if err := jl.logFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync log file: %w", err)
	}

	return nil
}

// CloseJSONLLogger closes the global JSONL logger
func CloseJSONLLogger() error {
	return closeGlobalJSONLLogger()
}

// LogRPCMessageJSONL logs an RPC message to the global JSONL logger
func LogRPCMessageJSONL(direction RPCMessageDirection, messageType RPCMessageType, serverID, method string, payloadBytes []byte, err error) {
	globalJSONLMu.RLock()
	defer globalJSONLMu.RUnlock()

	if globalJSONLLogger == nil {
		return
	}

	entry := &JSONLRPCMessage{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Direction: string(direction),
		Type:      string(messageType),
		ServerID:  serverID,
		Method:    method,
		Payload:   sanitize.SanitizeJSON(payloadBytes),
	}

	if err != nil {
		entry.Error = err.Error()
	}

	// Best effort logging - don't fail if JSONL logging fails
	_ = globalJSONLLogger.LogMessage(entry)
}
