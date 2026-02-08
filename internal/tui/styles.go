package tui

import "github.com/charmbracelet/lipgloss"

// Color palette
var (
	colorPrimary   = lipgloss.Color("170") // magenta/pink
	colorSecondary = lipgloss.Color("62")  // muted blue
	colorError     = lipgloss.Color("160") // red
	colorSuccess   = lipgloss.Color("78")  // green
	colorMuted     = lipgloss.Color("241") // gray
	colorUnread    = lipgloss.Color("203") // coral/red for unread badge
	colorDateRange = lipgloss.Color("117") // cyan for date range mode
)

// Layout styles
var (
	titleStyle      = lipgloss.NewStyle().MarginLeft(2).Bold(true)
	itemStyle       = lipgloss.NewStyle().PaddingLeft(4)
	selectedStyle   = lipgloss.NewStyle().PaddingLeft(2).Foreground(colorPrimary)
	helpStyle       = lipgloss.NewStyle().PaddingLeft(4).PaddingBottom(1).Foreground(colorMuted)
	quitTextStyle   = lipgloss.NewStyle().Margin(1, 0, 2, 4)
	errorStyle      = lipgloss.NewStyle().MarginLeft(2).Foreground(colorError)
	statusBarStyle  = lipgloss.NewStyle().PaddingLeft(2).Foreground(colorSecondary)
	infoStyle       = lipgloss.NewStyle().MarginLeft(4)
	emptyListStyle  = lipgloss.NewStyle().MarginLeft(2).Foreground(colorMuted).Italic(true)
	unreadBadge     = lipgloss.NewStyle().Foreground(colorUnread)
	modeUnreadStyle = lipgloss.NewStyle().Foreground(colorSuccess).Bold(true)
	modeDateStyle   = lipgloss.NewStyle().Foreground(colorDateRange).Bold(true)
)

// Chat type icons
const (
	iconPrivate = "👤"
	iconGroup   = "👥"
	iconChannel = "📢"
	iconForum   = "🗂"
	iconBot     = "🤖"
	iconTopic   = "💬"
)

// List dimensions
const (
	minListWidth      = 20
	minListHeight     = 6
	defaultListWidth  = 60
	defaultListHeight = 14
	dateFormat        = "2006-01-02"
)
