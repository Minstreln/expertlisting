package tests

import (
	"io"

	"github.com/sirupsen/logrus"
)

// newTestLogger returns a silent logger so test output is not cluttered.
func newTestLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(io.Discard)
	return log
}
