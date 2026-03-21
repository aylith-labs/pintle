package logger

import (
	"fmt"
	"time"
)

const (
	reset   = "\x1b[0m"
	dim     = "\x1b[2m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	cyan    = "\x1b[36m"
	red     = "\x1b[31m"
)

func timestamp() string {
	return time.Now().Format("15:04:05")
}

func Info(msg string) {
	fmt.Printf("%s%s%s %sINFO%s  %s\n", dim, timestamp(), reset, cyan, reset, msg)
}

func Infof(format string, args ...any) {
	Info(fmt.Sprintf(format, args...))
}

func Route(method, host, path, target string, status int) {
	color := green
	if status >= 400 && status < 500 {
		color = yellow
	} else if status >= 500 {
		color = red
	}
	fmt.Printf("%s%s%s %s%d%s   %s%s%s %s%s %s->%s %s\n",
		dim, timestamp(), reset,
		color, status, reset,
		dim, method, reset,
		host, path,
		dim, reset,
		target)
}

func RouteChange(action, hostname, path, target string) {
	symbol := green + "+"
	if action == "remove" {
		symbol = red + "-"
	}
	fmt.Printf("%s%s%s %s%s     %s%s %s->%s %s\n",
		dim, timestamp(), reset,
		symbol, reset,
		hostname, path,
		dim, reset,
		target)
}

func Warn(msg string) {
	fmt.Printf("%s%s%s %sWARN%s  %s\n", dim, timestamp(), reset, yellow, reset, msg)
}

func Warnf(format string, args ...any) {
	Warn(fmt.Sprintf(format, args...))
}

func Error(msg string, err error) {
	if err != nil {
		fmt.Printf("%s%s%s %sERROR%s %s: %v\n", dim, timestamp(), reset, red, reset, msg, err)
	} else {
		fmt.Printf("%s%s%s %sERROR%s %s\n", dim, timestamp(), reset, red, reset, msg)
	}
}

func Errorf(format string, args ...any) {
	fmt.Printf("%s%s%s %sERROR%s %s\n", dim, timestamp(), reset, red, reset, fmt.Sprintf(format, args...))
}
