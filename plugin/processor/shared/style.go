package shared

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"os"
	"strings"
)

var (
	renderer           = lipgloss.NewRenderer(os.Stdout, termenv.WithTTY(true))
	unchangedStyle     = lipgloss.NewStyle().Renderer(renderer).Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#eeeeee"))
	increaseStyle      = lipgloss.NewStyle().Renderer(renderer).Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#ee0000"))
	decreaseStyle      = lipgloss.NewStyle().Renderer(renderer).Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#00ee00"))
	notConfiguredStyle = lipgloss.NewStyle().Renderer(renderer).Background(lipgloss.Color("0")).Foreground(lipgloss.Color("#eeee00"))
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
