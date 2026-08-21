package cli

import (
	"os"
	"strings"
)

func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	st, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

func colorMethod(method string) string {
	method = strings.ToUpper(method)
	if !useColor() {
		return padMethod(method)
	}
	var code string
	switch method {
	case "GET", "HEAD":
		code = "32"
	case "POST":
		code = "36"
	case "PUT", "PATCH":
		code = "33"
	case "DELETE":
		code = "31"
	default:
		code = "35"
	}
	return "\x1b[" + code + "m" + padMethod(method) + "\x1b[0m"
}

func padMethod(method string) string {
	if len(method) >= 7 {
		return method
	}
	return method + strings.Repeat(" ", 7-len(method))
}

func dim(s string) string {
	if s == "" || !useColor() {
		return s
	}
	return "\x1b[2m" + s + "\x1b[0m"
}
