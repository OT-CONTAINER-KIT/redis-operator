package log

import "github.com/go-logr/logr"

type Logger interface {
	Enabled() bool
	Error(err error, msg string, keysAndValues ...interface{})
	GetSink() logr.LogSink
	Info(msg string, keysAndValues ...interface{})
	IsZero() bool
	V(level int) logr.Logger
	WithCallDepth(depth int) logr.Logger
	WithCallStackHelper() (func(), logr.Logger)
	WithName(name string) logr.Logger
	WithSink(sink logr.LogSink) logr.Logger
	WithValues(keysAndValues ...interface{}) logr.Logger
}
