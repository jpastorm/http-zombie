package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	zombieGreen  = lipgloss.Color("#7FFF00")
	zombieDark   = lipgloss.Color("#2D5016")
	bloodRed     = lipgloss.Color("#8B0000")
	boneWhite    = lipgloss.Color("#F5F5DC")
	ghostGray    = lipgloss.Color("#696969")
	rottenYellow = lipgloss.Color("#9B870C")
	toxicPurple  = lipgloss.Color("#6B3FA0")

	// HTTP status colors
	status2xx = lipgloss.Color("#98C379") // green
	status3xx = lipgloss.Color("#56B6C2") // cyan
	status4xx = lipgloss.Color("#E5C07B") // yellow
	status5xx = lipgloss.Color("#E06C75") // red

	// Header colors
	headerKeyColor = lipgloss.Color("#56B6C2") // muted cyan
	headerValColor = lipgloss.Color("#ABB2BF") // light gray
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(zombieGreen).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(zombieGreen).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(boneWhite)

	categoryStyle = lipgloss.NewStyle().
			Foreground(ghostGray).
			Italic(true)

	statusStyle = lipgloss.NewStyle().
			Foreground(rottenYellow).
			MarginTop(1)

	helpStyle = lipgloss.NewStyle().
			Foreground(ghostGray)

	errorStyle = lipgloss.NewStyle().
			Foreground(bloodRed).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(zombieGreen)

	responseHeaderStyle = lipgloss.NewStyle().
				Foreground(toxicPurple).
				Bold(true)

	dimStyle = lipgloss.NewStyle().
			Foreground(ghostGray)

	searchPromptStyle = lipgloss.NewStyle().
				Foreground(zombieGreen).
				Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(zombieDark).
			Padding(1, 2)

	// Header formatting styles
	headerKeyStyle = lipgloss.NewStyle().Foreground(headerKeyColor)
	headerValStyle = lipgloss.NewStyle().Foreground(headerValColor)

	// Response section styles
	sectionTitleStyle = lipgloss.NewStyle().
				Foreground(zombieGreen).
				Bold(true)

	sectionBorder = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#444444")).
			Padding(0, 1)

	metaKeyStyle = lipgloss.NewStyle().
			Foreground(ghostGray)

	metaValStyle = lipgloss.NewStyle().
			Foreground(boneWhite)

	// Modal styles
	modalBorder = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(zombieGreen).
			Padding(0, 1)

	modalTitleStyle = lipgloss.NewStyle().
			Foreground(zombieGreen).
			Bold(true).
			Align(lipgloss.Center)

	panelActiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(zombieGreen).
				Padding(0, 1)

	panelInactiveBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#444444")).
				Padding(0, 1)

	curlAccent = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E06C75")).
			Bold(true)

	xhAccent = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#61AFEF")).
			Bold(true)

	scrollIndicator = lipgloss.NewStyle().
			Foreground(ghostGray)

	// Button style
	buttonStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1a1a")).
			Background(zombieGreen).
			Bold(true).
			Padding(0, 2)

	buttonDimStyle = lipgloss.NewStyle().
			Foreground(ghostGray).
			Background(lipgloss.Color("#333333")).
			Padding(0, 2)

	// View mode tab styles
	activeTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1a1a")).
			Background(zombieGreen).
			Bold(true).
			Padding(0, 1)

	inactiveTabStyle = lipgloss.NewStyle().
				Foreground(ghostGray).
				Background(lipgloss.Color("#333333")).
				Padding(0, 1)

	// Method badge styles
	methodBadgeGET = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1a1a")).
			Background(status2xx).
			Bold(true).
			Padding(0, 1)
	methodBadgePOST = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1a1a1a")).
				Background(lipgloss.Color("#E5C07B")).
				Bold(true).
				Padding(0, 1)
	methodBadgePUT = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1a1a1a")).
			Background(status3xx).
			Bold(true).
			Padding(0, 1)
	methodBadgeDELETE = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1a1a1a")).
				Background(status4xx).
				Bold(true).
				Padding(0, 1)
	methodBadgeDefault = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#1a1a1a")).
				Background(ghostGray).
				Bold(true).
				Padding(0, 1)

	// Curl paste view styles
	curlOuterBorder = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(zombieGreen).
			Padding(0, 1)

	curlSectionTitle = lipgloss.NewStyle().
				Foreground(zombieGreen).
				Bold(true)

	curlPreviewBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#444444")).
				Padding(0, 1)

	curlURLStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5C07B")).
			Bold(true)

	curlBodySnippet = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#98C379"))

	curlFlagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#56B6C2")).
			Italic(true)
)
