package log

import (
	"fmt"
	"log"
	"os"
	"strings"
)

// StdLogger implements Logger using the standard library log package.
type StdLogger struct {
	l *log.Logger
}

// NewStdLogger constructs StdLogger writing to stderr.
func NewStdLogger() *StdLogger {
	return &StdLogger{
		l: log.New(os.Stderr, "", log.LstdFlags),
	}
}

func (s *StdLogger) Info(msg string, kv ...interface{}) {
	s.l.Println(formatMessage("INFO", msg, kv...))
}

func (s *StdLogger) Warn(msg string, kv ...interface{}) {
	s.l.Println(formatMessage("WARN", msg, kv...))
}

func (s *StdLogger) Error(msg string, kv ...interface{}) {
	s.l.Println(formatMessage("ERROR", msg, kv...))
}

func formatMessage(level, msg string, kv ...interface{}) string {
	if len(kv) == 0 {
		return fmt.Sprintf("[%s] %s", level, msg)
	}

	return fmt.Sprintf("[%s] %s | %s", level, msg, joinKeyValues(kv...))
}

func joinKeyValues(kv ...interface{}) string {
	if len(kv) == 0 {
		return ""
	}

	var builder strings.Builder

	for i := 0; i < len(kv); i += 2 {
		if i > 0 {
			builder.WriteString(" ")
		}

		key := fmt.Sprint(kv[i])

		var value interface{}

		if i+1 < len(kv) {
			value = kv[i+1]
		}

		builder.WriteString(fmt.Sprintf("%s=%v", key, value))
	}

	return builder.String()
}
