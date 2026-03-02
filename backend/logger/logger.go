package applog 

import (
	"log/slog"
	"os"
	"fmt"
)

// Wrapper around slog.Logger which implements telego.Logger's interface.
type Logger struct {
	*slog.Logger
}

// Returns a new logger (with slog.Logger as the base) which implements telego.Logger's 
// interface
func NewLogger() *Logger {

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	return &Logger{Logger: logger} 
}

// Logs debugging messages. Similar to Logger.Debug, but this is needed to satisfy 
// telego.Logger's interface
func (logger *Logger) Debugf (format string, args ...any) {
	logger.Debug(fmt.Sprintf(format, args...))
}

// Logs error messages. Similar to Logger.Error, but this is needed to satisfy 
// telego.Logger's interface
func (logger *Logger) Errorf (format string, args ...any) {
	logger.Error(fmt.Sprintf(format, args...))
}

