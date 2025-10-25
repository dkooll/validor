package validor

import (
	"github.com/fatih/color"
)

var redError = color.New(color.FgHiRed, color.Bold).SprintFunc()

func BoolToStr(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}
