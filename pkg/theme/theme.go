package theme

import "charm.land/lipgloss/v2"

// Locked Custom Color Scheme
const (
	ColorPrimaryBlue   = "#1557FF" // Electric Blue
	ColorAccentLime    = "#D7FF00" // Acid Lime
	ColorSecondaryPurp = "#D7A8FF" // Soft Lavender
	ColorBlack         = "#0B0B0D" // Rich Black
	ColorWhite         = "#F8F8F6" // Off White
)

// Lip Gloss Styles
var (
	// App Header
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorAccentLime)).
			Background(lipgloss.Color(ColorBlack)).
			Padding(0, 1)

	// Page Titles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorPrimaryBlue))

	// ASCII Logo Style (Identity managed separately)
	LogoStyle = lipgloss.NewStyle()

	// Highlight / Primary accent (e.g. active inputs, selected elements)
	AccentStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorPrimaryBlue))

	// Success / Complete status
	SuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAccentLime))

	// Warning / Error status
	ErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")) // Standard warning red

	// Muted helper text / secondary elements
	MutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSecondaryPurp))

	// Selected active row in tables / lists
	SelectedRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorBlack)).
				Background(lipgloss.Color(ColorPrimaryBlue))

	// Selected tab in layout navigation
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorAccentLime)).
			Underline(true)

	// Inactive tab in layout navigation
	InactiveTabStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSecondaryPurp))

	// Table headers
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorPrimaryBlue))

	// Horizontal separator lines
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSecondaryPurp))
)
