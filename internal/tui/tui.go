package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/james-see/gofindadomain/internal/checker"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#00D4AA")
	secondaryColor = lipgloss.Color("#FF6B6B")
	accentColor    = lipgloss.Color("#FFE66D")
	dimColor       = lipgloss.Color("#666666")
	bgColor        = lipgloss.Color("#1a1a2e")

	// Styles
	titleStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true).
			Padding(0, 1)

	availableStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")).
			Bold(true)

	takenStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	expiryStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	inputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1)

	helpStyle = lipgloss.NewStyle().
			Foreground(dimColor)

	resultStyle = lipgloss.NewStyle().
			Padding(0, 2)

	bannerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)
)

const banner = `
  ___      ___ _         _   _   ___                  _      
 / __|___ | __(_)_ _  __| | /_\ |   \ ___ _ __  __ _(_)_ _  
| (_ / _ \| _|| | ' \/ _' |/ _ \| |) / _ \ '  \/ _' | | ' \ 
 \___\___/|_| |_|_||_\__,_/_/ \_\___/\___/_|_|_\__,_|_|_||_|
                   Domain Availability Checker
`

type state int

const (
	stateInput state = iota
	stateSelectTLDs
	stateChecking
	stateResults
)

type Model struct {
	state         state
	keywordInput  textinput.Model
	keyword       string
	tlds          []string
	selectedTLDs  map[int]bool
	tldCursor     int
	results       []checker.Result
	showOnlyAvail bool
	ctx           context.Context
	cancel        context.CancelFunc
	checking      bool
	checkedCount  int
	totalCount    int
	err           error
	width         int
	height        int
}

type resultMsg checker.Result
type checkDoneMsg struct {
	results []checker.Result
}

func NewModel(tlds []string) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter keyword (e.g., mycompany)"
	ti.Focus()
	ti.CharLimit = 63
	ti.Width = 40

	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		state:        stateInput,
		keywordInput: ti,
		tlds:         tlds,
		selectedTLDs: make(map[int]bool),
		ctx:          ctx,
		cancel:       cancel,
		width:        80,
		height:       24,
	}
}

func (m Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit

		case "tab":
			if m.state == stateResults {
				m.showOnlyAvail = !m.showOnlyAvail
			}
			return m, nil

		case "r":
			if m.state == stateResults {
				// Restart
				m.state = stateInput
				m.results = nil
				m.checkedCount = 0
				m.keywordInput.Focus()
				return m, textinput.Blink
			}
		}

		switch m.state {
		case stateInput:
			switch msg.String() {
			case "enter":
				m.keyword = m.keywordInput.Value()
				if m.keyword != "" {
					m.state = stateSelectTLDs
					// Don't pre-select any TLDs - let user choose
				}
				return m, nil
			}
			m.keywordInput, cmd = m.keywordInput.Update(msg)
			return m, cmd

		case stateSelectTLDs:
			switch msg.String() {
			case "up", "k":
				if m.tldCursor > 0 {
					m.tldCursor--
				}
			case "down", "j":
				if m.tldCursor < len(m.tlds)-1 {
					m.tldCursor++
				}
			case " ":
				m.selectedTLDs[m.tldCursor] = !m.selectedTLDs[m.tldCursor]
			case "a":
				// Toggle all
				allSelected := len(m.selectedTLDs) == len(m.tlds)
				m.selectedTLDs = make(map[int]bool)
				if !allSelected {
					for i := range m.tlds {
						m.selectedTLDs[i] = true
					}
				}
			case "p":
				// Select popular TLDs
				popular := []string{".com", ".net", ".org", ".io", ".dev", ".co", ".app", ".ai"}
				for i, tld := range m.tlds {
					for _, p := range popular {
						if tld == p {
							m.selectedTLDs[i] = true
						}
					}
				}
			case "enter":
				if len(m.selectedTLDs) > 0 {
					m.state = stateChecking
					m.checking = true
					m.totalCount = len(m.selectedTLDs)
					return m, m.startChecking()
				}
			case "backspace", "esc":
				m.state = stateInput
				m.keywordInput.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case stateChecking:
			// Allow cancel during checking
			return m, nil

		case stateResults:
			switch msg.String() {
			case "up", "k":
				// Could add scrolling here
			case "down", "j":
				// Could add scrolling here
			}
			return m, nil
		}

	case resultMsg:
		m.results = append(m.results, checker.Result(msg))
		m.checkedCount++
		return m, nil

	case checkDoneMsg:
		m.checking = false
		m.state = stateResults
		m.results = msg.results
		m.checkedCount = len(msg.results)
		return m, nil
	}

	return m, nil
}

func (m Model) startChecking() tea.Cmd {
	var domains []string
	for i, selected := range m.selectedTLDs {
		if selected {
			domains = append(domains, m.keyword+m.tlds[i])
		}
	}

	ctx := m.ctx

	return func() tea.Msg {
		resultChan := make(chan checker.Result, len(domains))

		go func() {
			checker.CheckDomains(ctx, domains, 30, resultChan)
			close(resultChan)
		}()

		var results []checker.Result
		for result := range resultChan {
			results = append(results, result)
		}

		return checkDoneMsg{results: results}
	}
}

func (m Model) View() string {
	var s strings.Builder

	s.WriteString(bannerStyle.Render(banner))
	s.WriteString("\n")

	switch m.state {
	case stateInput:
		s.WriteString(titleStyle.Render("Enter a keyword to search:"))
		s.WriteString("\n\n")
		s.WriteString(inputStyle.Render(m.keywordInput.View()))
		s.WriteString("\n\n")
		s.WriteString(helpStyle.Render("Press Enter to continue • Ctrl+C to quit"))

	case stateSelectTLDs:
		s.WriteString(titleStyle.Render(fmt.Sprintf("Select TLDs for '%s':", m.keyword)))
		s.WriteString("\n\n")

		// Show a scrollable list of TLDs
		visibleCount := min(m.height-12, len(m.tlds))
		start := max(0, m.tldCursor-visibleCount/2)
		end := min(len(m.tlds), start+visibleCount)
		if end-start < visibleCount && start > 0 {
			start = max(0, end-visibleCount)
		}

		for i := start; i < end; i++ {
			cursor := "  "
			if i == m.tldCursor {
				cursor = "▸ "
			}
			checked := "[ ]"
			if m.selectedTLDs[i] {
				checked = "[✓]"
			}
			line := fmt.Sprintf("%s%s %s", cursor, checked, m.tlds[i])
			if m.selectedTLDs[i] {
				s.WriteString(availableStyle.Render(line))
			} else {
				s.WriteString(line)
			}
			s.WriteString("\n")
		}

		s.WriteString("\n")
		s.WriteString(helpStyle.Render(fmt.Sprintf("Selected: %d • Space: toggle • 'a': all • 'p': popular (.com,.net,.org,.io,.dev,.co,.app,.ai) • Enter: check", len(m.selectedTLDs))))

	case stateChecking:
		s.WriteString(titleStyle.Render("Checking domains..."))
		s.WriteString("\n\n")
		s.WriteString(fmt.Sprintf("Progress: %d/%d\n", m.checkedCount, m.totalCount))
		s.WriteString("\n")

		// Show results as they come in
		for _, r := range m.results {
			s.WriteString(formatResult(r, m.showOnlyAvail))
		}

		s.WriteString("\n")
		s.WriteString(helpStyle.Render("Press Ctrl+C to cancel"))

	case stateResults:
		s.WriteString(titleStyle.Render("Results:"))
		if m.showOnlyAvail {
			s.WriteString(helpStyle.Render(" (showing available only)"))
		}
		s.WriteString("\n\n")

		availCount := 0
		for _, r := range m.results {
			if r.Available {
				availCount++
			}
			s.WriteString(formatResult(r, m.showOnlyAvail))
		}

		s.WriteString("\n")
		s.WriteString(fmt.Sprintf("Total: %d checked • %d available • %d taken\n",
			len(m.results), availCount, len(m.results)-availCount))
		s.WriteString("\n")
		s.WriteString(helpStyle.Render("Tab to toggle filter • 'r' to restart • 'q' to quit"))
	}

	return s.String()
}

func formatResult(r checker.Result, showOnlyAvail bool) string {
	if r.Error != nil {
		return fmt.Sprintf("[error] %s - %v\n", r.Domain, r.Error)
	}

	if r.Available {
		return availableStyle.Render("[avail]") + " " + r.Domain + "\n"
	}

	if showOnlyAvail {
		return ""
	}

	if r.ExpiryDate != "" {
		return takenStyle.Render("[taken]") + " " + r.Domain + " - Exp: " + expiryStyle.Render(r.ExpiryDate) + "\n"
	}
	return takenStyle.Render("[taken]") + " " + r.Domain + "\n"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Run starts the TUI
func Run(tlds []string) error {
	p := tea.NewProgram(NewModel(tlds), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// RunWithUpdates runs the TUI with a channel for live updates
func RunWithUpdates(tlds []string) error {
	model := NewModel(tlds)
	p := tea.NewProgram(model, tea.WithAltScreen())

	_, err := p.Run()
	return err
}
