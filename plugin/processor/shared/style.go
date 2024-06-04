package shared

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

var (
	unchangedStyle     = lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#eeeeee"))
	increaseStyle      = lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#ee0000"))
	decreaseStyle      = lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#00ee00"))
	notConfiguredStyle = lipgloss.NewStyle().Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#eeee00"))
)

func SprintfWithStyle(format string, value float64, notConfigured bool) string {
	str := format
	if strings.Contains(format, "%") {
		str = fmt.Sprintf(format, value)
	}
	if notConfigured {
		str = notConfiguredStyle.Render(str)
	} else if value < 0 {
		str = decreaseStyle.Render(str)
	} else if value > 0 {
		str = increaseStyle.Render(str)
	} else {
		str = unchangedStyle.Render(str)
	}
	return str
}
