package voxlattice

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type logLevel int

const (
	levelDebug logLevel = iota
	levelInfo
	levelWarn
	levelError
)

type appLogger struct {
	min    logLevel
	logger *log.Logger
}

type rotatingFileWriter struct {
	mu   sync.Mutex
	file *os.File
	size int64
	max  int64
}

var appLog = newAppLogger(levelWarn, os.Stdout)

func (l logLevel) String() string {
	switch l {
	case levelDebug:
		return "debug"
	case levelInfo:
		return "info"
	case levelWarn:
		return "warn"
	case levelError:
		return "error"
	default:
		return "warn"
	}
}

func parseLogLevel(s string) (logLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return levelDebug, nil
	case "info":
		return levelInfo, nil
	case "warn", "warning":
		return levelWarn, nil
	case "error":
		return levelError, nil
	default:
		return levelWarn, fmt.Errorf("invalid log level: %s", s)
	}
}

func newAppLogger(level logLevel, out io.Writer) *appLogger {
	return &appLogger{
		min:    level,
		logger: log.New(out, "", log.LstdFlags),
	}
}

func (l *appLogger) logf(level logLevel, format string, args ...interface{}) {
	if l == nil || level < l.min {
		return
	}
	prefix := level.String()
	l.logger.Printf("[%s] "+format, append([]interface{}{prefix}, args...)...)
}

func (l *appLogger) Debugf(format string, args ...interface{}) { l.logf(levelDebug, format, args...) }
func (l *appLogger) Infof(format string, args ...interface{})  { l.logf(levelInfo, format, args...) }
func (l *appLogger) Warnf(format string, args ...interface{})  { l.logf(levelWarn, format, args...) }
func (l *appLogger) Errorf(format string, args ...interface{}) { l.logf(levelError, format, args...) }
func (l *appLogger) Fatalf(format string, args ...interface{}) {
	l.logf(levelError, format, args...)
	os.Exit(1)
}

func newRotatingFileWriter(path string, maxBytes int64) (*rotatingFileWriter, error) {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	size := info.Size()
	if size >= maxBytes {
		if err := file.Truncate(0); err != nil {
			_ = file.Close()
			return nil, err
		}
		if _, err := file.Seek(0, 0); err != nil {
			_ = file.Close()
			return nil, err
		}
		size = 0
	}
	return &rotatingFileWriter{
		file: file,
		size: size,
		max:  maxBytes,
	}, nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, fmt.Errorf("log file not available")
	}
	if w.size+int64(len(p)) > w.max {
		if err := w.file.Truncate(0); err != nil {
			return 0, err
		}
		if _, err := w.file.Seek(0, 0); err != nil {
			return 0, err
		}
		w.size = 0
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func initLogger(path string, level logLevel) error {
	var out io.Writer = os.Stdout
	if path != "" {
		writer, err := newRotatingFileWriter(path, defaultLogMaxBytes)
		if err != nil {
			return err
		}
		out = writer
	}
	appLog = newAppLogger(level, out)
	return nil
}
