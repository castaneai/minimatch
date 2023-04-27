package mmlog

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger interface {
	Debugf(template string, args ...interface{})
	Infof(template string, args ...interface{})
	Warnf(template string, args ...interface{})
	Errorf(template string, args ...interface{})
	Fatalf(template string, args ...interface{})
}

var defaultLogger *DefaultLogger

func init() {
	config := zap.NewDevelopmentConfig()
	config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapLogger, _ := config.Build(
		zap.AddCallerSkip(2),
		zap.AddStacktrace(zap.FatalLevel))
	defaultLogger = &DefaultLogger{base: zapLogger.Sugar()}
}

func Debugf(template string, args ...interface{}) {
	defaultLogger.Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
	defaultLogger.Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	defaultLogger.Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	defaultLogger.Errorf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
	defaultLogger.Fatalf(template, args...)
}

type DefaultLogger struct {
	base *zap.SugaredLogger
}

func (l *DefaultLogger) Debugf(template string, args ...interface{}) {
	l.base.Debugf(template, args...)
}

func (l *DefaultLogger) Infof(template string, args ...interface{}) {
	l.base.Infof(template, args...)
}

func (l *DefaultLogger) Warnf(template string, args ...interface{}) {
	l.base.Warnf(template, args...)
}

func (l *DefaultLogger) Errorf(template string, args ...interface{}) {
	l.base.Errorf(template, args...)
}

func (l *DefaultLogger) Fatalf(template string, args ...interface{}) {
	l.base.Fatalf(template, args...)
}
