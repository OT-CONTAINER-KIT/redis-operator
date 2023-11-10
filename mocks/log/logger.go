package log

type LoggerInterface interface {
	Error(error, string, ...interface{})
}
