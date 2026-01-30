// Package log provides configurable logging for dnstm.
package log

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/net2share/dnstm/internal/config"
)

// Level defines the log level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var levelNames = map[Level]string{
	LevelDebug: "DEBUG",
	LevelInfo:  "INFO",
	LevelWarn:  "WARN",
	LevelError: "ERROR",
}

var levelFromString = map[string]Level{
	"debug": LevelDebug,
	"info":  LevelInfo,
	"warn":  LevelWarn,
	"error": LevelError,
}

// Logger provides configurable logging.
type Logger struct {
	mu        sync.Mutex
	level     Level
	output    io.Writer
	file      *os.File
	timestamp bool
}

var defaultLogger = &Logger{
	level:     LevelInfo,
	output:    os.Stderr,
	timestamp: true,
}

// Configure sets up the logger from config.
func Configure(cfg *config.LogConfig) error {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()

	// Set level
	if cfg.Level != "" {
		level, ok := levelFromString[cfg.Level]
		if !ok {
			return fmt.Errorf("invalid log level: %s", cfg.Level)
		}
		defaultLogger.level = level
	}

	// Set timestamp
	if cfg.Timestamp != nil {
		defaultLogger.timestamp = *cfg.Timestamp
	}

	// Set output
	if cfg.Output != "" {
		// Close previous file if any
		if defaultLogger.file != nil {
			defaultLogger.file.Close()
		}

		// Open log file
		f, err := os.OpenFile(cfg.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file: %w", err)
		}
		defaultLogger.file = f
		defaultLogger.output = io.MultiWriter(os.Stderr, f)
	} else {
		defaultLogger.output = os.Stderr
	}

	return nil
}

// SetLevel sets the log level.
func SetLevel(level Level) {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	defaultLogger.level = level
}

// SetLevelString sets the log level from a string.
func SetLevelString(level string) error {
	l, ok := levelFromString[level]
	if !ok {
		return fmt.Errorf("invalid log level: %s", level)
	}
	SetLevel(l)
	return nil
}

// Close closes any open log file.
func Close() {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	if defaultLogger.file != nil {
		defaultLogger.file.Close()
		defaultLogger.file = nil
	}
}

func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)

	if l.timestamp {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(l.output, "[%s] [%s] %s\n", timestamp, levelNames[level], msg)
	} else {
		fmt.Fprintf(l.output, "[%s] %s\n", levelNames[level], msg)
	}
}

// Debug logs a debug message.
func Debug(format string, args ...interface{}) {
	defaultLogger.log(LevelDebug, format, args...)
}

// Info logs an info message.
func Info(format string, args ...interface{}) {
	defaultLogger.log(LevelInfo, format, args...)
}

// Warn logs a warning message.
func Warn(format string, args ...interface{}) {
	defaultLogger.log(LevelWarn, format, args...)
}

// Error logs an error message.
func Error(format string, args ...interface{}) {
	defaultLogger.log(LevelError, format, args...)
}

// Debugf is an alias for Debug.
func Debugf(format string, args ...interface{}) {
	Debug(format, args...)
}

// Infof is an alias for Info.
func Infof(format string, args ...interface{}) {
	Info(format, args...)
}

// Warnf is an alias for Warn.
func Warnf(format string, args ...interface{}) {
	Warn(format, args...)
}

// Errorf is an alias for Error.
func Errorf(format string, args ...interface{}) {
	Error(format, args...)
}

// IsDebugEnabled returns true if debug logging is enabled.
func IsDebugEnabled() bool {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	return defaultLogger.level <= LevelDebug
}

// GetLevel returns the current log level.
func GetLevel() Level {
	defaultLogger.mu.Lock()
	defer defaultLogger.mu.Unlock()
	return defaultLogger.level
}

// GetLevelString returns the current log level as a string.
func GetLevelString() string {
	return levelNames[GetLevel()]
}

// ParseLevel parses a log level string.
func ParseLevel(s string) (Level, error) {
	level, ok := levelFromString[s]
	if !ok {
		return LevelInfo, fmt.Errorf("invalid log level: %s", s)
	}
	return level, nil
}
