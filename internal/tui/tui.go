package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/jobot2397/linker/internal/config"
	"github.com/jobot2397/linker/internal/symlink"
)

// Semantic palette — color + symbol so status stays readable without color.
var (
	colAccent  = lipgloss.Color("212")
	colOk      = lipgloss.Color("42")
	colWarn    = lipgloss.Color("214")
	colErr     = lipgloss.Color("203")
	colMuted   = lipgloss.Color("243")
	colSubtle  = lipgloss.Color("238")
	colText    = lipgloss.Color("252")
	colBorder  = lipgloss.Color("238")
	colSelect  = lipgloss.Color("63")

	styleTitle   = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleMuted   = lipgloss.NewStyle().Foreground(colMuted)
	styleSubtle  = lipgloss.NewStyle().Foreground(colSubtle)
	styleText    = lipgloss.NewStyle().Foreground(colText)
	styleOk      = lipgloss.NewStyle().Foreground(colOk).Bold(true)
	styleWarn    = lipgloss.NewStyle().Foreground(colWarn).Bold(true)
	styleErr     = lipgloss.NewStyle().Foreground(colErr).Bold(true)
	styleCursor  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleHelp    = lipgloss.NewStyle().Foreground(colMuted)
	styleHeader  = lipgloss.NewStyle().Foreground(colMuted).Bold(true)
	stylePanel   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(colBorder).Padding(0, 1)
	styleSelRow  = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(colText)
)

type phase int

const (
	phaseLinked phase = iota // on disk & wanted
	phaseOff                 // absent & not wanted
	phaseWillLink            // wanted, not yet
	phaseWillUnlink          // linked, user wants off
	phaseConflict            // dest exists but isn't our link
	phaseMissingSrc          // source path missing
)

func (p phase) badge() (symbol, label string, style lipgloss.Style) {
	switch p {
	case phaseLinked:
		return "●", "LINKED", styleOk
	case phaseOff:
		return "○", "OFF", styleMuted
	case phaseWillLink:
		return "◐", "WILL LINK", styleWarn
	case phaseWillUnlink:
		return "◐", "WILL UNLINK", styleWarn
	case phaseConflict:
		return "✕", "CONFLICT", styleErr
	case phaseMissingSrc:
		return "!", "NO SOURCE", styleErr
	default:
		return "?", "UNKNOWN", styleMuted
	}
}

type linkRow struct {
	link       config.Link
	src        string
	dst        string
	wantLinked bool
	actual     symlink.Status
	srcExists  bool
	srcIsDir   bool
}

func (r linkRow) phase() phase {
	if !r.srcExists {
		return phaseMissingSrc
	}
	if r.actual.Exists && !r.actual.Linked {
		return phaseConflict
	}
	if r.wantLinked && r.actual.Linked {
		return phaseLinked
	}
	if !r.wantLinked && !r.actual.Linked {
		return phaseOff
	}
	if r.wantLinked && !r.actual.Linked {
		return phaseWillLink
	}
	return phaseWillUnlink
}

func (r linkRow) planLine() string {
	switch r.phase() {
	case phaseLinked:
		return "Already correct — apply will leave this alone."
	case phaseOff:
		return "Disabled — apply will leave destination absent."
	case phaseWillLink:
		return fmt.Sprintf("Apply will create:  %s  →  %s", short(r.dst), short(r.src))
	case phaseWillUnlink:
		return fmt.Sprintf("Apply will remove symlink at %s", short(r.dst))
	case phaseConflict:
		return fmt.Sprintf("Blocked: %s already exists (%s). linker never overwrites.", short(r.dst), r.actual.Message)
	case phaseMissingSrc:
		return fmt.Sprintf("Blocked: source missing at %s", short(r.src))
	default:
		return ""
	}
}

type applyDoneMsg struct {
	outcomes []symlink.Outcome
}

type model struct {
	rows     []linkRow
	cursor   int
	repoRoot string
	cfgPath  string
	width    int
	height   int
	applying bool
	status   string
	errMsg   string
	quitting bool
}

// Run launches the interactive TUI for the given config.
func Run(cfg *config.Config, repoRoot, cfgPath string) error {
	rows := make([]linkRow, 0, len(cfg.Links))
	for _, link := range cfg.Links {
		src, dst, err := link.Resolve(repoRoot)
		if err != nil {
			return err
		}
		st := symlink.Inspect(src, dst)
		info, err := os.Lstat(src)
		row := linkRow{
			link:       link,
			src:        src,
			dst:        dst,
			wantLinked: st.Linked,
			actual:     st,
			srcExists:  err == nil,
		}
		if err == nil {
			row.srcIsDir = info.IsDir()
		}
		rows = append(rows, row)
	}

	m := model{
		rows:     rows,
		repoRoot: repoRoot,
		cfgPath:  cfgPath,
		status:   "Review links, toggle what you want, then press a to apply.",
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
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
				errs = append(errs, fmt.Sprintf("%s: %s", short(o.Destination), o.Message))
			}
		}
		m.refresh()
		m.status = fmt.Sprintf("Applied — created %d · removed %d · unchanged %d · failed %d",
			created, removed, skipped, failed)
		if failed > 0 {
			m.errMsg = strings.Join(errs, " · ")
		} else {
			m.errMsg = ""
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit

		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil

		case "down", "j":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
			return m, nil

		case " ", "enter":
			if len(m.rows) == 0 {
				return m, nil
			}
			r := &m.rows[m.cursor]
			// Conflicts / missing source can still be "wanted" but apply will skip.
			r.wantLinked = !r.wantLinked
			m.status = m.rows[m.cursor].planLine()
			m.errMsg = ""
			return m, nil

		case "A":
			for i := range m.rows {
				m.rows[i].wantLinked = true
			}
			m.status = "All marked to link — press a to apply."
			return m, nil

		case "D":
			for i := range m.rows {
				m.rows[i].wantLinked = false
			}
			m.status = "All marked to unlink — press a to apply."
			return m, nil

		case "r":
			m.refresh()
			m.status = "Refreshed filesystem status."
			m.errMsg = ""
			return m, nil

		case "a":
			if m.applying {
				return m, nil
			}
			m.applying = true
			m.status = "Applying symlink changes…"
			m.errMsg = ""
			return m, m.applyCmd()
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 {
		m.width = 80
	}
	if m.height == 0 {
		m.height = 24
	}

	header := m.viewHeader()
	list := m.viewList()
	detail := m.viewDetail()
	footer := m.viewFooter()

	// Budget vertical space: header + footer fixed, split remainder list/detail ~60/40.
	chrome := lipgloss.Height(header) + lipgloss.Height(footer) + 2
	remain := m.height - chrome
	if remain < 8 {
		remain = 8
	}
	listH := remain * 3 / 5
	if listH < 5 {
		listH = 5
	}
	detailH := remain - listH
	if detailH < 6 {
		detailH = 6
		listH = remain - detailH
		if listH < 3 {
			listH = 3
		}
	}

	listPanel := stylePanel.Width(m.width - 2).Height(listH).Render(list)
	detailPanel := stylePanel.Width(m.width - 2).Height(detailH).Render(detail)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		listPanel,
		detailPanel,
		footer,
	)
}

func (m model) viewHeader() string {
	linked, pending, conflict, off := 0, 0, 0, 0
	for _, r := range m.rows {
		switch r.phase() {
		case phaseLinked:
			linked++
		case phaseWillLink, phaseWillUnlink:
			pending++
		case phaseConflict, phaseMissingSrc:
			conflict++
		case phaseOff:
			off++
		}
	}

	title := styleTitle.Render("linker")
	counts := styleMuted.Render(fmt.Sprintf(
		"%d links  %s  %s  %s  %s",
		len(m.rows),
		styleOk.Render(fmt.Sprintf("● %d linked", linked)),
		styleWarn.Render(fmt.Sprintf("◐ %d pending", pending)),
		styleErr.Render(fmt.Sprintf("✕ %d blocked", conflict)),
		styleMuted.Render(fmt.Sprintf("○ %d off", off)),
	))
	cfg := styleSubtle.Render(short(m.cfgPath))

	left := title + "  " + counts
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(cfg) - 2
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + cfg
}

func (m model) viewList() string {
	var b strings.Builder
	b.WriteString(styleHeader.Render(fmt.Sprintf("  %-12s  %-28s  %s  %s", "STATUS", "REPO SOURCE", "→", "HOME DESTINATION")))
	b.WriteString("\n")
	b.WriteString(styleSubtle.Render("  " + strings.Repeat("─", max(20, m.width-8))))
	b.WriteString("\n")

	if len(m.rows) == 0 {
		b.WriteString(styleMuted.Render("  No links in config."))
		return b.String()
	}

	srcW := 28
	dstW := max(20, m.width-12-12-srcW-6)

	for i, r := range m.rows {
		sym, label, st := r.phase().badge()
		badge := st.Render(fmt.Sprintf("%s %-10s", sym, label))
		src := truncate(r.link.Source, srcW)
		dst := truncate(r.link.Destination, dstW)
		arrow := styleMuted.Render("→")

		cursor := " "
		line := fmt.Sprintf("%s %s  %-*s  %s  %s", cursor, badge, srcW, src, arrow, dst)
		if i == m.cursor {
			cursor = styleCursor.Render("▸")
			line = styleSelRow.Width(max(10, m.width-6)).Render(
				fmt.Sprintf("%s %s  %-*s  %s  %s", cursor, badge, srcW, src, "→", dst),
			)
		} else {
			line = fmt.Sprintf("  %s  %-*s  %s  %s", badge, srcW, src, arrow, styleText.Render(dst))
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m model) viewDetail() string {
	var b strings.Builder
	b.WriteString(styleHeader.Render("Selected link — what will happen"))
	b.WriteString("\n")

	if len(m.rows) == 0 {
		b.WriteString(styleMuted.Render("Nothing selected."))
		return b.String()
	}

	r := m.rows[m.cursor]
	sym, label, st := r.phase().badge()

	b.WriteString(st.Render(fmt.Sprintf("%s %s", sym, label)))
	b.WriteString("  ")
	b.WriteString(styleText.Render(r.planLine()))
	b.WriteString("\n\n")

	// Visual flow: source ═══▸ destination
	srcKind := "file"
	if r.srcIsDir {
		srcKind = "dir"
	}
	srcState := styleErr.Render("missing")
	if r.srcExists {
		srcState = styleOk.Render("exists · "+srcKind)
	}

	dstState := styleMuted.Render("absent")
	switch {
	case r.actual.Linked:
		dstState = styleOk.Render("symlink → " + short(r.src))
	case r.actual.IsSymlink:
		dstState = styleWarn.Render("symlink → " + short(r.actual.Target) + " (other)")
	case r.actual.Exists:
		dstState = styleErr.Render("real file/dir (not a symlink)")
	}

	flow := lipgloss.JoinHorizontal(lipgloss.Top,
		m.box("REPO (source)", short(r.src), srcState),
		styleMuted.Render("\n\n  ═══▸  "),
		m.box("HOME (destination)", short(r.dst), dstState),
	)
	b.WriteString(flow)
	return b.String()
}

func (m model) box(title, path, state string) string {
	inner := styleHeader.Render(title) + "\n" +
		styleText.Render(truncate(path, 36)) + "\n" +
		state
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(min(40, max(24, (m.width-16)/2))).
		Render(inner)
}

func (m model) viewFooter() string {
	var b strings.Builder
	if m.errMsg != "" {
		b.WriteString(styleErr.Render("✗ "+truncate(m.errMsg, m.width-4)) + "\n")
	} else if m.status != "" {
		s := styleOk
		if m.applying {
			s = styleMuted
		} else if strings.HasPrefix(m.status, "Blocked") || strings.Contains(m.status, "pending") {
			s = styleWarn
		}
		b.WriteString(s.Render("→ "+truncate(m.status, m.width-4)) + "\n")
	}
	b.WriteString(styleHelp.Render("space/enter toggle · a apply · A link-all · D unlink-all · r refresh · q quit"))
	return b.String()
}

func (m *model) refresh() {
	for i := range m.rows {
		r := &m.rows[i]
		r.actual = symlink.Inspect(r.src, r.dst)
		info, err := os.Lstat(r.src)
		r.srcExists = err == nil
		if err == nil {
			r.srcIsDir = info.IsDir()
		}
	}
}

func (m model) applyCmd() tea.Cmd {
	rows := append([]linkRow(nil), m.rows...)
	return func() tea.Msg {
		outcomes := make([]symlink.Outcome, 0, len(rows))
		for _, r := range rows {
			outcomes = append(outcomes, symlink.ApplyDesired(r.src, r.dst, r.wantLinked))
		}
		return applyDoneMsg{outcomes: outcomes}
	}
}

func short(path string) string {
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		if path == home {
			return "~"
		}
		if strings.HasPrefix(path, home+string(filepath.Separator)) {
			return "~" + path[len(home):]
		}
	}
	return path
}

func truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
