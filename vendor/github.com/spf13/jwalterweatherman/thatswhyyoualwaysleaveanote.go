// Copyright © 2014 Steve Francia <spf@spf13.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package jwalterweatherman

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
)

// Level describes the chosen log level between
// debug and critical.
type Level int

type NotePad struct {
	Handle io.Writer
	Level  Level
	Prefix string
	Logger **log.Logger
}

// Feedback is special. It writes plainly to the output while
// logging with the standard extra information (date, file, etc)
// Only Println and Printf are currently provided for this
type Feedback struct{}

const (
	LevelTrace Level = iota
	LevelDebug
	LevelInfo
	LevelWarn
	LevelError
	LevelCritical
	LevelFatal
	DefaultLogThreshold    = LevelWarn
	DefaultStdoutThreshold = LevelError
)

var (
	TRACE      *log.Logger
	DEBUG      *log.Logger
	INFO       *log.Logger
	WARN       *log.Logger
	ERROR      *log.Logger
	CRITICAL   *log.Logger
	FATAL      *log.Logger
	LOG        *log.Logger
	FEEDBACK   Feedback
	LogHandle  io.Writer  = ioutil.Discard
	OutHandle  io.Writer  = os.Stdout
	BothHandle io.Writer  = io.MultiWriter(LogHandle, OutHandle)
	NotePads   []*NotePad = []*NotePad{trace, debug, info, warn, err, critical, fatal}

	trace           *NotePad = &NotePad{Level: LevelTrace, Handle: os.Stdout, Logger: &TRACE, Prefix: "TRACE: "}
	debug           *NotePad = &NotePad{Level: LevelDebug, Handle: os.Stdout, Logger: &DEBUG, Prefix: "DEBUG: "}
	info            *NotePad = &NotePad{Level: LevelInfo, Handle: os.Stdout, Logger: &INFO, Prefix: "INFO: "}
	warn            *NotePad = &NotePad{Level: LevelWarn, Handle: os.Stdout, Logger: &WARN, Prefix: "WARN: "}
	err             *NotePad = &NotePad{Level: LevelError, Handle: os.Stdout, Logger: &ERROR, Prefix: "ERROR: "}
	critical        *NotePad = &NotePad{Level: LevelCritical, Handle: os.Stdout, Logger: &CRITICAL, Prefix: "CRITICAL: "}
	fatal           *NotePad = &NotePad{Level: LevelFatal, Handle: os.Stdout, Logger: &FATAL, Prefix: "FATAL: "}
	logThreshold    Level    = DefaultLogThreshold
	outputThreshold Level    = DefaultStdoutThreshold
)

func init() {
	SetStdoutThreshold(DefaultStdoutThreshold)
}

// initialize will setup the jWalterWeatherman standard approach of providing the user
// some feedback and logging a potentially different amount based on independent log and output thresholds.
// By default the output has a lower threshold than logged
// Don't use if you have manually set the Handles of the different levels as it will overwrite them.
func initialize() {
	BothHandle = io.MultiWriter(LogHandle, OutHandle)

	for _, n := range NotePads {
		if n.Level < outputThreshold && n.Level < logThreshold {
			n.Handle = ioutil.Discard
		} else if n.Level >= outputThreshold && n.Level >= logThreshold {
			n.Handle = BothHandle
		} else if n.Level >= outputThreshold && n.Level < logThreshold {
			n.Handle = OutHandle
		} else {
			n.Handle = LogHandle
		}
	}

	for _, n := range NotePads {
		*n.Logger = log.New(n.Handle, n.Prefix, log.Ldate)
	}

	LOG = log.New(LogHandle,
		"LOG:   ",
		log.Ldate|log.Ltime|log.Lshortfile)
}

// Level returns the current global log threshold.
func LogThreshold() Level {
	return logThreshold
}

// Level returns the current global output threshold.
func StdoutThreshold() Level {
	return outputThreshold
}

// Ensures that the level provided is within the bounds of available levels
func levelCheck(level Level) Level {
	switch {
	case level <= LevelTrace:
		return LevelTrace
	case level >= LevelFatal:
		return LevelFatal
	default:
		return level
	}
}

// Establishes a threshold where anything matching or above will be logged
func SetLogThreshold(level Level) {
	logThreshold = levelCheck(level)
	initialize()
}

// Establishes a threshold where anything matching or above will be output
func SetStdoutThreshold(level Level) {
	outputThreshold = levelCheck(level)
	initialize()
}

// Conveniently Sets the Log Handle to a io.writer created for the file behind the given filepath
// Will only append to this file
func SetLogFile(path string) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		CRITICAL.Println("Failed to open log file:", path, err)
		os.Exit(-1)
	}
        fmt.Println("Logging to", file.Name())

	LogHandle = file
	initialize()
}

// Conveniently Creates a temporary file and sets the Log Handle to a io.writer created for it
func UseTempLogFile(prefix string) {
	file, err := ioutil.TempFile(os.TempDir(), prefix)
	if err != nil {
		CRITICAL.Println(err)
	}

	fmt.Println("Logging to", file.Name())

	LogHandle = file
	initialize()
}

// Disables logging for the entire JWW system
func DiscardLogging() {
	LogHandle = ioutil.Discard
	initialize()
}

// Feedback is special. It writes plainly to the output while
// logging with the standard extra information (date, file, etc)
// Only Println and Printf are currently provided for this
func (fb *Feedback) Println(v ...interface{}) {
	fmt.Println(v...)
	LOG.Println(v...)
}

// Feedback is special. It writes plainly to the output while
// logging with the standard extra information (date, file, etc)
// Only Println and Printf are currently provided for this
func (fb *Feedback) Printf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
	LOG.Printf(format, v...)
}
