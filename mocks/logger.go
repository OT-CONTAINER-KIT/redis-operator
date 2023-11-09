package mocks

type LoggerInterface interface {
	Error(error, string, ...interface{})
}
