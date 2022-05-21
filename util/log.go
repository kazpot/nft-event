package util

import (
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"os"
)

type Log struct{}

func NewLog() *Log {
	return &Log{}
}

func (l *Log) SetUp(config *Config, level log.Level) *os.File {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
	log.SetLevel(level)
	log.SetReportCaller(true)

	var file *os.File
	if config.LogOutput {

		logName := config.LogName
		if _, err := os.Stat(logName); errors.Is(err, os.ErrNotExist) {
			file, err = os.Create(logName)
			if err != nil {
				log.Fatal(err)
			}
		} else {
			file, err = os.OpenFile(logName, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
			if err != nil {
				log.Fatal(err)
			}
		}

		log.SetOutput(io.MultiWriter(file, os.Stdout))
	}
	return file
}
