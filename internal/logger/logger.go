package logger

// Logger exposes structured logging capabilities required by the application.
type Logger interface {
	Info(msg string, kv ...interface{})
	Warn(msg string, kv ...interface{})
	Error(msg string, kv ...interface{})
}
