// Package log provides functions that are mainly wrappers for github.com/go-kit/log,
// but help provide consistent log behavior within the project.
//
// In particular, this package focuses on providing level-based structured logging with
// the following standardized fields:
//
//   - "msg": The main log message
//   - "ts": Timestamp formatted with time.RFC3339Nano
//   - "caller": The file and line number (as "file.go:N") that emitted the log
//   - "error": (ERROR-level logs only) The error that serves as the reason for the log
package log

import (
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type Logger log.Logger

// ConfigureLogger configures the Logger pointer with a level-based filter that emits JSON
// structured logs.
// lvl may be one of: DEBUG, INFO, WARN, ERROR and is not case-sensitive.
// The configured logger wil emit logs that always include a "ts"-keyed timestamp value
// and "caller"-keyed string value in the form "filename.go:lineno" that references where
// the log occurred.
func ConfigureLogger(l *Logger, lvl string) {
	*l = log.With(
		level.NewFilter(
			log.NewJSONLogger(os.Stderr),
			level.Allow(level.ParseDefault(lvl, level.InfoValue())),
		),
		"ts", log.DefaultTimestamp,
		"caller", log.Caller(5),
	)
}

// With is a wrapper for log.With() and exists to provide brevity/syntactic sugar
func With(logger Logger, keyvals ...interface{}) log.Logger {
	return log.With(logger, keyvals...)
}

// Debug logs a message and any keyvals with DEBUG level
func Debug(l Logger, msg interface{}, kv ...interface{}) {
	logWithMessage(level.Debug(l), msg, kv...)
}

// Info logs a message and any keyvals with INFO level
func Info(l Logger, msg interface{}, kv ...interface{}) {
	logWithMessage(level.Info(l), msg, kv...)
}

// Warn logs a message and any keyvals with WARN level
func Warn(l Logger, msg interface{}, kv ...interface{}) {
	logWithMessage(level.Warn(l), msg, kv...)
}

// Error logs a message, error and any keyvals with ERROR level
func Error(l Logger, msg interface{}, err error, kv ...interface{}) {
	logWithMessage(level.Error(log.With(l, "error", err)), msg, kv...)
}

// Errorf is like Error() but returns a new error that wraps err with msg.
// It exists to provide brevity/syntactic sugar by allowing callers to handle and log errors
// in a DRY manner, as demonstrated in the following example:
//
//	// myFuncA is functionally equivalent myFuncB
//	func myFuncA(thing string) error {
//		err := doSomething(thing)
//		if err != nil {
//			log.Error(logger, "Error doing something", err, "thing", thing)
//			return fmt.Errorf("Error doing something: %w", err)
//		}
//		return nil
//	}
//
//	// myFuncB is functionally equivalent to myFuncA
//	func myFuncB(thing string) error {
//		err := doSomething(thing)
//		if err != nil {
//			return log.Errorf(logger, "Error doing something", err, "thing", thing)
//		}
//		return nil
//	}
//
// Note that kvs are included in the log output, but not in the returned error.
func Errorf(l Logger, msg interface{}, err error, kv ...interface{}) error {
	logWithMessage(level.Error(log.With(l, "error", err)), msg, kv...)
	return fmt.Errorf("%s: %w", msg, err)
}

func logWithMessage(l Logger, msg interface{}, kv ...interface{}) {
	log.With(l, "msg", msg).Log(kv...)
}
