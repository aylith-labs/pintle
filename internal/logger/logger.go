package logger

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	reset  = "\x1b[0m"
	dim    = "\x1b[2m"
	green  = "\x1b[32m"
	yellow = "\x1b[33m"
	cyan   = "\x1b[36m"
	red    = "\x1b[31m"
)

var (
	outputMu sync.Mutex
	output   io.Writer = os.Stdout
)

// SetOutput redirects every log line to w and returns a function that restores the previous writer.
func SetOutput(w io.Writer) (restore func()) {
	outputMu.Lock()
	previous := output
	output = w
	outputMu.Unlock()
	return func() {
		outputMu.Lock()
		output = previous
		outputMu.Unlock()
	}
}

// printf writes one whole line under the lock, so concurrent lines never interleave.
func printf(format string, args ...any) {
	outputMu.Lock()
	defer outputMu.Unlock()
	fmt.Fprintf(output, format, args...)
}

func timestamp() string {
	return time.Now().Format("15:04:05")
}

func Info(msg string) {
	printf("%s%s%s %sINFO%s  %s\n", dim, timestamp(), reset, cyan, reset, msg)
}

func Infof(format string, args ...any) {
	Info(fmt.Sprintf(format, args...))
}

func statusColor(status int) string {
	switch {
	case status >= 500:
		return red
	case status >= 400:
		return yellow
	default:
		return green
	}
}

// Route logs one request as "<status>   <METHOD> host/path -> target".
func Route(method, host, path, target string, status int) {
	printf("%s%s%s %s%d%s   %s%s%s %s%s %s->%s %s\n",
		dim, timestamp(), reset,
		statusColor(status), status, reset,
		dim, method, reset,
		host, path,
		dim, reset,
		target)
}

// Access logs one finished request in the Route shape, followed by its duration, the response body
// size, and "aborted" when the response ended before it was complete.
func Access(method, host, path, target string, status int, elapsed time.Duration, size int64, aborted bool) {
	color := statusColor(status)
	suffix := ""
	if aborted {
		color = red
		suffix = " " + red + "aborted" + reset
	}
	printf("%s%s%s %s%d%s   %s%s%s %s%s %s->%s %s %s%s %s%s%s\n",
		dim, timestamp(), reset,
		color, status, reset,
		dim, method, reset,
		host, path,
		dim, reset,
		target,
		dim, FormatDuration(elapsed), FormatBytes(size), reset,
		suffix)
}

// FormatDuration renders a request duration with a precision that suits its size.
func FormatDuration(elapsed time.Duration) string {
	switch {
	case elapsed < time.Millisecond:
		return strconv.FormatInt(elapsed.Microseconds(), 10) + "µs"
	case elapsed < time.Second:
		return strconv.FormatFloat(float64(elapsed.Microseconds())/1000, 'f', 1, 64) + "ms"
	default:
		return strconv.FormatFloat(elapsed.Seconds(), 'f', 2, 64) + "s"
	}
}

// FormatBytes renders a byte count in B, KB or MB (powers of 1024).
func FormatBytes(size int64) string {
	switch {
	case size < 1024:
		return strconv.FormatInt(size, 10) + "B"
	case size < 1024*1024:
		return strconv.FormatFloat(float64(size)/1024, 'f', 1, 64) + "KB"
	default:
		return strconv.FormatFloat(float64(size)/(1024*1024), 'f', 1, 64) + "MB"
	}
}

func RouteChange(action, hostname, path, target string) {
	symbol := green + "+"
	if action == "remove" {
		symbol = red + "-"
	}
	printf("%s%s%s %s%s     %s%s %s->%s %s\n",
		dim, timestamp(), reset,
		symbol, reset,
		hostname, path,
		dim, reset,
		target)
}

func Warn(msg string) {
	printf("%s%s%s %sWARN%s  %s\n", dim, timestamp(), reset, yellow, reset, msg)
}

func Warnf(format string, args ...any) {
	Warn(fmt.Sprintf(format, args...))
}

func Error(msg string, err error) {
	if err != nil {
		printf("%s%s%s %sERROR%s %s: %v\n", dim, timestamp(), reset, red, reset, msg, err)
	} else {
		printf("%s%s%s %sERROR%s %s\n", dim, timestamp(), reset, red, reset, msg)
	}
}

func Errorf(format string, args ...any) {
	printf("%s%s%s %sERROR%s %s\n", dim, timestamp(), reset, red, reset, fmt.Sprintf(format, args...))
}
