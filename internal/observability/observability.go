package observability

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

var (
	defaultOnce    sync.Once
	defaultLogger  *Logger
	defaultMetrics *Metrics
)

// Logger emits structured log lines to stderr.
type Logger struct {
	base *log.Logger
}

// Metrics tracks minimal counters for adapter and repository activity.
type Metrics struct {
	mu       sync.Mutex
	counters map[string]int64
}

// DefaultLogger returns a shared logger instance.
func DefaultLogger() *Logger {
	initDefaults()
	return defaultLogger
}

// DefaultMetrics returns a shared metrics instance.
func DefaultMetrics() *Metrics {
	initDefaults()
	return defaultMetrics
}

// NewLogger constructs a logger that writes to stderr.
func NewLogger() *Logger {
	return &Logger{base: log.New(os.Stderr, "", log.LstdFlags)}
}

// NewMetrics constructs a metrics collector.
func NewMetrics() *Metrics {
	return &Metrics{counters: make(map[string]int64)}
}

// Info logs an informational message.
func (l *Logger) Info(message string, fields map[string]any) {
	l.log("info", message, fields)
}

// Error logs an error message.
func (l *Logger) Error(message string, fields map[string]any) {
	l.log("error", message, fields)
}

// Inc increments a named counter.
func (m *Metrics) Inc(name string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counters[name]++
}

// Snapshot returns a copy of the current counters.
func (m *Metrics) Snapshot() map[string]int64 {
	if m == nil {
		return map[string]int64{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := make(map[string]int64, len(m.counters))
	for key, value := range m.counters {
		snapshot[key] = value
	}
	return snapshot
}

func initDefaults() {
	defaultOnce.Do(func() {
		defaultLogger = NewLogger()
		defaultMetrics = NewMetrics()
	})
}

func (l *Logger) log(level string, message string, fields map[string]any) {
	if l == nil || l.base == nil {
		return
	}
	var builder strings.Builder
	builder.WriteString("level=")
	builder.WriteString(level)
	builder.WriteString(" msg=")
	builder.WriteString(strconv.Quote(message))
	for key, value := range fields {
		builder.WriteString(" ")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(formatValue(value))
	}
	l.base.Print(builder.String())
}

func formatValue(value any) string {
	switch typed := value.(type) {
	case string:
		return strconv.Quote(typed)
	case fmt.Stringer:
		return strconv.Quote(typed.String())
	default:
		return fmt.Sprintf("%v", typed)
	}
}
