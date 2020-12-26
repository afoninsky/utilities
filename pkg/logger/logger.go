package logger

import "github.com/sirupsen/logrus"

type Logger struct {
	*logrus.Logger
}

func New() *Logger {
	log := Logger{
		Logger: logrus.New(),
	}
	return &log
}

func (l *Logger) FatalfIfFalse(flag bool, format string, args ...interface{}) {
	if !flag {
		l.Fatalf(format, args...)
	}
}

func (l *Logger) FatalIfErr(err error) {
	if err != nil {
		l.Fatal(err)
	}
}
