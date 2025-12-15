package tui

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
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

	bannerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	spinnerStyle = lipgloss.NewStyle().Foreground(primaryColor)
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

// Shared state for async results
type asyncResults struct {
	mu       sync.Mutex
	results  []checker.Result
	done     bool
}

var sharedResults *asyncResults

type Model struct {
	state         state
	keywordInput  textinput.Model
	spinner       spinner.Model
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
	startTime     time.Time
	err           error
	width         int
	height        int
}

type tickMsg time.Time
type checkDoneMsg struct{}

func NewModel(tlds []string) Model {
	ti := textinput.New()
	ti.Placeholder = "Enter keyword (e.g., mycompany)"
	ti.Focus()
	ti.CharLimit = 63
	ti.Width = 40

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = spinnerStyle

	ctx, cancel := context.WithCancel(context.Background())

	return Model{
		state:        stateInput,
		keywordInput: ti,
		spinner:      s,
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

func tickEvery() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
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
				allSelected := len(m.selectedTLDs) == len(m.tlds)
				m.selectedTLDs = make(map[int]bool)
				if !allSelected {
					for i := range m.tlds {
						m.selectedTLDs[i] = true
					}
				}
			case "p":
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
					m.startTime = time.Now()
					return m, tea.Batch(m.startChecking(), m.spinner.Tick, tickEvery())
				}
			case "backspace", "esc":
				m.state = stateInput
				m.keywordInput.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case stateChecking:
			return m, nil

		case stateResults:
			return m, nil
		}

	case spinner.TickMsg:
		if m.state == stateChecking {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tickMsg:
		if m.state != stateChecking || sharedResults == nil {
			return m, nil
		}

		sharedResults.mu.Lock()
		m.results = make([]checker.Result, len(sharedResults.results))
		copy(m.results, sharedResults.results)
		m.checkedCount = len(m.results)
		done := sharedResults.done
		sharedResults.mu.Unlock()

		if done {
			m.checking = false
			m.state = stateResults
			return m, nil
		}

		return m, tea.Batch(tickEvery(), m.spinner.Tick)

	case checkDoneMsg:
		m.checking = false
		m.state = stateResults
		if sharedResults != nil {
			sharedResults.mu.Lock()
			m.results = sharedResults.results
			m.checkedCount = len(m.results)
			sharedResults.mu.Unlock()
		}
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

	// Initialize shared results
	sharedResults = &asyncResults{
		results: make([]checker.Result, 0, len(domains)),
	}

	return func() tea.Msg {
		resultChan := make(chan checker.Result, len(domains))

		go func() {
			checker.CheckDomains(ctx, domains, 30, resultChan)
			close(resultChan)
		}()

		for result := range resultChan {
			sharedResults.mu.Lock()
			sharedResults.results = append(sharedResults.results, result)
			sharedResults.mu.Unlock()
		}

		sharedResults.mu.Lock()
		sharedResults.done = true
		sharedResults.mu.Unlock()

		return checkDoneMsg{}
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
		s.WriteString(helpStyle.Render(fmt.Sprintf("Selected: %d • Space: toggle • 'a': all • 'p': popular • Enter: check", len(m.selectedTLDs))))

	case stateChecking:
		s.WriteString(m.spinner.View())
		s.WriteString(titleStyle.Render(" Checking domains..."))
		s.WriteString("\n\n")

		// Progress bar
		pct := 0
		if m.totalCount > 0 {
			pct = (m.checkedCount * 100) / m.totalCount
		}
		barWidth := 40
		filled := (pct * barWidth) / 100
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		elapsed := time.Since(m.startTime).Round(time.Second)

		s.WriteString(fmt.Sprintf("Progress: [%s] %d/%d (%d%%) - %s\n\n", bar, m.checkedCount, m.totalCount, pct, elapsed))

		// Show last few results
		if len(m.results) > 0 {
			s.WriteString(helpStyle.Render("Recent results:\n"))
			start := max(0, len(m.results)-5)
			for _, r := range m.results[start:] {
				s.WriteString(formatResult(r, false))
			}
		}

		s.WriteString("\n")
		s.WriteString(helpStyle.Render("Press Ctrl+C to cancel"))

	case stateResults:
		elapsed := time.Since(m.startTime).Round(time.Second)
		s.WriteString(titleStyle.Render(fmt.Sprintf("Results (completed in %s):", elapsed)))
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
