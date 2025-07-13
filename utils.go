package validor

import (
	"strings"

	"github.com/fatih/color"
)

var redError = color.New(color.FgHiRed, color.Bold).SprintFunc()

// parseExceptionList converts comma-separated exception string to a map
func parseExceptionList() {
	exceptionList = make(map[string]bool)
	if exception != "" {
		examples := strings.Split(exception, ",")
		for _, ex := range examples {
			exceptionList[strings.TrimSpace(ex)] = true
		}
	}
}

// BoolToStr returns yes if condition is true, otherwise returns no
func BoolToStr(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}
