// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package clog provides global conditional logging for application components.
package clog

import (
	"fmt"
	"log"
)

var enabled = false

// Enable turns on conditional log output.
func Enable() {
	enabled = true
}

// A CLogger represents a logger object that logs output in the manner of the
// standard logger but can be conditionally enabled. By default, conditional
// logging is disabled.
type CLogger struct {
	logger *log.Logger // standard logger with prefix
}

// New creates a new conditional logger with the given prefix.
func New(prefixFormat string, prefixArgs ...any) *CLogger {
	return &CLogger{
		log.New(
			log.Default().Writer(),
			fmt.Sprintf(prefixFormat, prefixArgs...),
			log.Ldate|log.Ltime|log.Lmicroseconds|log.Lmsgprefix,
		),
	}
}

// Printf logs output conditionally (if enabled with -l command line option) in
// the manner of log.Printf.
func (c *CLogger) Printf(format string, a ...any) {
	if !enabled {
		return
	}
	c.logger.Printf(format, a...)
}

// Errorf logs output unconditionally, i.e. always, in the manner of log.Printf.
func (c *CLogger) Errorf(format string, a ...any) {
	c.logger.Printf(format, a...)
}
