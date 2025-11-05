package logger

import (
	"github.com/sirupsen/logrus"
)

var Log *logrus.Logger

func InitLog(loglevel string) *logrus.Logger {
	logger := logrus.New()
	switch {
	case loglevel == "debug":
		logger.Level = logrus.DebugLevel
	case loglevel == "info":
		logger.Level = logrus.InfoLevel
	case loglevel == "error":
		logger.Level = logrus.ErrorLevel
	case loglevel == "fatal":
		logger.Level = logrus.FatalLevel
	case loglevel == "warn":
		logger.Level = logrus.WarnLevel
	case loglevel == "panic":
		logger.Level = logrus.PanicLevel
	case loglevel == "trace":
		logger.Level = logrus.TraceLevel
	default:
		logger.Level = logrus.InfoLevel
	}
	logger.Formatter = &logrus.TextFormatter{
		FullTimestamp: true,
		DisableColors: false,
		ForceColors: true,
	}
	Log = logger

	return logger
}
