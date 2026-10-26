package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jobot2397/linker/internal/config"
	"github.com/jobot2397/linker/internal/symlink"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	statusOn      = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	statusOff     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusDirty   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	successStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
)

// linkItem is one row in the TUI list.
type linkItem struct {
	link        config.Link
	src         string
	dst         string
	wantLinked  bool // desired state (toggled by user)
	actual      symlink.Status
	lastMessage string
}

func (i linkItem) Title() string {
	mark := statusOff.Render("[ ]")
	if i.wantLinked {
		mark = statusOn.Render("[✓]")
	}
	dirty := ""
	if i.wantLinked != i.actual.Linked {
		dirty = statusDirty.Render(" *")
	}
	return fmt.Sprintf("%s %s%s", mark, i.link.Source, dirty)
}

func (i linkItem) Description() string {
	state := i.actual.Message
	return fmt.Sprintf("%s → %s  (%s)", i.src, i.dst, state)
}

func (i linkItem) FilterValue() string {
	return i.link.Source + " " + i.link.Destination
}

type applyDoneMsg struct {
	outcomes []symlink.Outcome
}

type model struct {
	list     list.Model
	repoRoot string
	cfgPath  string
	width    int
	height   int
	applying bool
	status   string
	errMsg   string
	quitting bool
}

type keyMap struct {
	Toggle key.Binding
	Apply  key.Binding
	AllOn  key.Binding
	AllOff key.Binding
	Quit   key.Binding
	Refresh key.Binding
}

var keys = keyMap{
	Toggle: key.NewBinding(
		key.WithKeys(" ", "enter"),
		key.WithHelp("space/enter", "toggle"),
	),
	Apply: key.NewBinding(
		key.WithKeys("a"),
		key.WithHelp("a", "apply changes"),
	),
	AllOn: key.NewBinding(
		key.WithKeys("A"),
		key.WithHelp("A", "enable all"),
	),
	AllOff: key.NewBinding(
		key.WithKeys("D"),
		key.WithHelp("D", "disable all"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh status"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// Run launches the interactive TUI for the given config.
func Run(cfg *config.Config, repoRoot, cfgPath string) error {
	items := make([]list.Item, 0, len(cfg.Links))
	for _, link := range cfg.Links {
		src, dst, err := link.Resolve(repoRoot)
		if err != nil {
			return err
		}
		st := symlink.Inspect(src, dst)
		items = append(items, linkItem{
			link:       link,
			src:        src,
			dst:        dst,
			wantLinked: st.Linked, // start from current reality
			actual:     st,
		})
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	delegate.SetSpacing(1)

	l := list.New(items, delegate, 0, 0)
	l.Title = "linker — toggle symlinks"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(true)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Toggle, keys.Apply, keys.Quit}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.Toggle, keys.Apply, keys.AllOn, keys.AllOff, keys.Refresh, keys.Quit}
	}

	m := model{
		list:     l,
		repoRoot: repoRoot,
		cfgPath:  cfgPath,
		status:   fmt.Sprintf("loaded %s", cfgPath),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.SetSize(msg.Width, msg.Height-3)
		return m, nil

	case applyDoneMsg:
		m.applying = false
		created, removed, skipped, failed := 0, 0, 0, 0
		var errs []string
		for _, o := range msg.outcomes {
			switch o.Result {
			case symlink.Created:
				created++
			case symlink.Removed:
				removed++
			case symlink.Skipped, symlink.AlreadyCorrect:
				skipped++
			case symlink.Failed:
				failed++
				errs = append(errs, o.Message)
			}
		}
		m.refreshActual()
		m.status = fmt.Sprintf("applied: %d created, %d removed, %d unchanged, %d failed",
			created, removed, skipped, failed)
		if failed > 0 {
			m.errMsg = strings.Join(errs, "; ")
		} else {
			m.errMsg = ""
		}
		return m, nil

	case tea.KeyMsg:
		if m.list.FilterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.Toggle):
			if i, ok := m.list.SelectedItem().(linkItem); ok {
				i.wantLinked = !i.wantLinked
				m.setItem(m.list.Index(), i)
			}
			return m, nil

		case key.Matches(msg, keys.AllOn):
			m.setAllDesired(true)
			return m, nil

		case key.Matches(msg, keys.AllOff):
			m.setAllDesired(false)
			return m, nil

		case key.Matches(msg, keys.Refresh):
			m.refreshActual()
			m.status = "status refreshed"
			m.errMsg = ""
			return m, nil

		case key.Matches(msg, keys.Apply):
			if m.applying {
				return m, nil
			}
			m.applying = true
			m.status = "applying…"
			m.errMsg = ""
			return m, m.applyCmd()
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.list.View())
	b.WriteString("\n")
	if m.errMsg != "" {
		b.WriteString(errorStyle.Render("✗ "+m.errMsg) + "\n")
	} else if m.status != "" {
		style := successStyle
		if m.applying {
			style = dimStyle
		}
		b.WriteString(style.Render("→ "+m.status) + "\n")
	}
	b.WriteString(helpStyle.Render("space toggle · a apply · A/D all on/off · r refresh · q quit"))
	return b.String()
}

func (m *model) setItem(idx int, item linkItem) {
	items := m.list.Items()
	if idx < 0 || idx >= len(items) {
		return
	}
	items[idx] = item
	m.list.SetItems(items)
}

func (m *model) setAllDesired(want bool) {
	items := m.list.Items()
	for i := range items {
		if li, ok := items[i].(linkItem); ok {
			li.wantLinked = want
			items[i] = li
		}
	}
	m.list.SetItems(items)
	if want {
		m.status = "all enabled (not applied yet)"
	} else {
		m.status = "all disabled (not applied yet)"
	}
}

func (m *model) refreshActual() {
	items := m.list.Items()
	for i := range items {
		if li, ok := items[i].(linkItem); ok {
			li.actual = symlink.Inspect(li.src, li.dst)
			// Keep wantLinked as-is; only sync display of actual state.
			items[i] = li
		}
	}
	m.list.SetItems(items)
}

func (m model) applyCmd() tea.Cmd {
	items := m.list.Items()
	return func() tea.Msg {
		outcomes := make([]symlink.Outcome, 0, len(items))
		for _, it := range items {
			li, ok := it.(linkItem)
			if !ok {
				continue
			}
			outcomes = append(outcomes, symlink.ApplyDesired(li.src, li.dst, li.wantLinked))
		}
		return applyDoneMsg{outcomes: outcomes}
	}
}
