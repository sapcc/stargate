package log

import (
	"os"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// Logger ...
type Logger struct {
	logger log.Logger
}

// NewLogger creates a new Logger
func NewLogger() Logger {
	var l log.Logger
	l = log.NewLogfmtLogger(os.Stdout)
	l = level.NewFilter(l, level.AllowAll())
	l = log.With(l, "ts", log.DefaultTimestampUTC, "caller", log.Caller(4))

	return Logger{
		logger: l,
	}
}

// NewLoggerWith return a new Logger with additional keyvals
func NewLoggerWith(logger Logger, keyvals ...interface{}) Logger {
	return Logger{
		logger: log.With(logger.logger, keyvals),
	}
}

// LogInfo logs info messages
func (l *Logger) LogInfo(msg string, keyvals ...interface{}) {
	level.Info(l.logger).Log("msg", msg, keyvals)
}

// LogDebug logs debug messages
func (l *Logger) LogDebug(msg string, keyvals ...interface{}) {
	level.Debug(l.logger).Log("msg", msg, keyvals)
}

// LogError logs error messages
func (l *Logger) LogError(msg string, err error, keyvals ...interface{}) {
	level.Error(l.logger).Log("msg", msg, "err", err, keyvals)
}

// LogWarn logs warning messages
func (l *Logger) LogWarn(msg string, keyvals ...interface{}) {
	level.Warn(l.logger).Log("msg", msg, keyvals)
}

// LogFatal logs fatal messages and exits
func (l *Logger) LogFatal(msg string, keyvals ...interface{}) {
	level.Error(l.logger).Log("msg", msg, keyvals)
	os.Exit(1)
}
