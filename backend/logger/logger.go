package applog

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
)

// Wrapper around slog.Logger which implements telego.Logger's interface.
type Logger struct {
	*slog.Logger
}

type StructuredError struct {
	Err  error
	Args []any
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
func (logger *Logger) Debugf(format string, args ...any) {
	logger.Debug(fmt.Sprintf(format, args...))
}

// Logs error messages. Similar to Logger.Error, but this is needed to satisfy
// telego.Logger's interface
func (logger *Logger) Errorf(format string, args ...any) {
	logger.Error(fmt.Sprintf(format, args...))
}

func (logger *Logger) Error(msg string, args ...any) {
	var message string = fmt.Sprintf("%s: ", msg)
	var structuredArgs []any

	for _, arg := range args {

		if err, ok := arg.(error); ok {
			message = fmt.Sprintf("%s: %s", message, err.Error())
			structuredArgs = append(structuredArgs, logger.extractArgs(err))
		} else {
			message = fmt.Sprintf("%s: %s", message, arg)
		}
	}

	logger.Logger.Error(message, structuredArgs...)
}

func (logger *Logger) extractArgs(err error) []any {

	var structuredArgs []any

	for err != nil {

		if sErr, ok := err.(*StructuredError); ok {
			structuredArgs = append(structuredArgs, sErr.Args...)
		} else {
			structuredArgs = append(structuredArgs, err.Error())
		}

		err = errors.Unwrap(err)
	}

	return structuredArgs
}

func (s *StructuredError) Error() string {
	return s.Err.Error()
}

func (s *StructuredError) Unwrap() error {
	return s.Err
}

func Wrap(err error, args ...any) *StructuredError {
	return &StructuredError{
		Err:  err,
		Args: args,
	}
}
