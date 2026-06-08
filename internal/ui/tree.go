package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/everythingwebza/claude-cpr/internal/data"
	"github.com/sahilm/fuzzy"
)

type RowKind int

const (
	RowProject RowKind = iota
	RowSession
)

type Row struct {
	Kind    RowKind
	Project string
	Session data.SessionInfo
	Active  bool
}

type SortMode string

const (
	SortRecent   SortMode = "recent"
	SortMsgCount SortMode = "msgcount"
	SortAlpha    SortMode = "alpha"
)

type TreeModel struct {
	sessions   []data.SessionInfo
	expanded   map[string]bool
	activeDirs map[string]struct{}
	pinned     map[string]bool
	sort       SortMode
	cursor     int
	filter     string

	scrollOffset  int // first visible row index — adjusted to keep cursor in view
	width, height int
}

func NewTreeModel(sessions []data.SessionInfo, expanded map[string]bool,
	activeDirs map[string]struct{}, sort SortMode) TreeModel {
	if expanded == nil {
		expanded = map[string]bool{}
	}
	return TreeModel{
		sessions: sessions, expanded: expanded, activeDirs: activeDirs, sort: sort,
		pinned: map[string]bool{},
	}
}

// flatten produces the visible rows given the current filter, expansion, and sort.
func (m TreeModel) flatten(filter string) []Row {
	// group by project
	grouped := map[string][]data.SessionInfo{}
	order := []string{}
	for _, s := range m.sessions {
		if _, ok := grouped[s.Project]; !ok {
			order = append(order, s.Project)
		}
		grouped[s.Project] = append(grouped[s.Project], s)
	}

	// apply filter at the session level: keep matching sessions; keep parent.
	if filter != "" {
		f := strings.ToLower(filter)
		for proj, sess := range grouped {
			kept := []data.SessionInfo{}
			for _, s := range sess {
				hay := strings.ToLower(proj + " " + s.Title)
				if matches := fuzzy.Find(f, []string{hay}); len(matches) > 0 {
					kept = append(kept, s)
				}
			}
			if len(kept) == 0 {
				delete(grouped, proj)
			} else {
				grouped[proj] = kept
			}
		}
		// rebuild order to drop empty projects (fresh allocation, not in-place
		// reuse — `order[:0]` would silently overwrite live entries during
		// concurrent reads in test code).
		newOrder := make([]string, 0, len(order))
		for _, p := range order {
			if _, ok := grouped[p]; ok {
				newOrder = append(newOrder, p)
			}
		}
		order = newOrder
	}

	// sort sessions within each project
	for proj, sess := range grouped {
		sortSessions(sess, m.sort)
		grouped[proj] = sess
	}

	// sort projects: pinned first, then by latest session Modified desc
	sort.SliceStable(order, func(i, j int) bool {
		ip := m.pinned[order[i]]
		jp := m.pinned[order[j]]
		if ip != jp {
			return ip
		}
		return latestModified(grouped[order[i]]) > latestModified(grouped[order[j]])
	})

	rows := []Row{}
	for _, proj := range order {
		_, active := m.activeDirs[proj]
		rows = append(rows, Row{Kind: RowProject, Project: proj, Active: active})
		if filter != "" || m.expanded[proj] {
			for _, s := range grouped[proj] {
				rows = append(rows, Row{Kind: RowSession, Project: proj, Session: s, Active: active})
			}
		}
	}
	return rows
}

func sortSessions(s []data.SessionInfo, mode SortMode) {
	switch mode {
	case SortMsgCount:
		sort.SliceStable(s, func(i, j int) bool { return s[i].MsgCount > s[j].MsgCount })
	case SortAlpha:
		sort.SliceStable(s, func(i, j int) bool { return strings.ToLower(s[i].Title) < strings.ToLower(s[j].Title) })
	default:
		sort.SliceStable(s, func(i, j int) bool { return s[i].Modified > s[j].Modified })
	}
}

func latestModified(s []data.SessionInfo) string {
	if len(s) == 0 {
		return ""
	}
	out := s[0].Modified
	for _, x := range s {
		if x.Modified > out {
			out = x.Modified
		}
	}
	return out
}

// SetFilter recomputes nothing yet; the View call uses the current filter.
func (m *TreeModel) SetFilter(filter string) {
	m.filter = filter
	m.cursor = 0
	m.scrollOffset = 0
}

// Update handles tree-pane keys. Returns the (possibly mutated) model and any cmd.
func (m TreeModel) Update(msg tea.Msg, keys KeyMap) (TreeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		rows := m.flatten(m.filter)
		switch {
		case key.Matches(msg, keys.Up):
			if m.cursor > 0 {
				m.cursor--
			}
		case key.Matches(msg, keys.Down):
			if m.cursor < len(rows)-1 {
				m.cursor++
			}
		case key.Matches(msg, keys.Right):
			if m.cursor < len(rows) && rows[m.cursor].Kind == RowProject {
				m.expanded[rows[m.cursor].Project] = true
			}
		case key.Matches(msg, keys.Left):
			if m.cursor < len(rows) && rows[m.cursor].Kind == RowSession {
				// collapse parent and move cursor to it
				target := rows[m.cursor].Project
				m.expanded[target] = false
				for i, r := range m.flatten(m.filter) {
					if r.Kind == RowProject && r.Project == target {
						m.cursor = i
						break
					}
				}
			} else if m.cursor < len(rows) && rows[m.cursor].Kind == RowProject {
				m.expanded[rows[m.cursor].Project] = false
			}
		}
		m.adjustScroll()
	}
	return m, nil
}

// adjustScroll keeps the cursor visible inside the [scrollOffset, scrollOffset+height)
// window. Called after any cursor or expansion change.
func (m *TreeModel) adjustScroll() {
	if m.height <= 0 {
		return
	}
	rows := m.flatten(m.filter)
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(rows) && len(rows) > 0 {
		m.cursor = len(rows) - 1
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+m.height {
		m.scrollOffset = m.cursor - m.height + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

// SelectedRow returns the currently-cursored row, or zero-value Row if none.
func (m TreeModel) SelectedRow() Row {
	rows := m.flatten(m.filter)
	if m.cursor < 0 || m.cursor >= len(rows) {
		return Row{}
	}
	return rows[m.cursor]
}

// View renders the tree, clipped to the visible window starting at scrollOffset
// and at most m.height rows tall. Lipgloss's Style.Height is a minimum (it
// pads but doesn't truncate), so the clip MUST happen here or the pane will
// overflow its bounds and push other UI off-screen.
//
// TODO(perf): View, SelectedRow, and Update each call flatten independently.
// Task 10's wiring may want to memoize the result and invalidate on
// filter/sort/sessions/expansion change.
func (m TreeModel) View() string {
	rows := m.flatten(m.filter)
	if len(rows) == 0 {
		return ""
	}

	visible := m.height
	if visible <= 0 {
		visible = len(rows) // fallback before SetSize is called (e.g., tests)
	}

	start := m.scrollOffset
	if start < 0 {
		start = 0
	}
	if start >= len(rows) {
		start = len(rows) - 1
	}
	end := start + visible
	if end > len(rows) {
		end = len(rows)
	}

	var b strings.Builder
	for i := start; i < end; i++ {
		line := m.renderRow(rows[i])
		if i == m.cursor {
			line = StyleSelected.Render(line)
		}
		// Truncate to the pane's content width so a too-long row never wraps
		// onto a second terminal line (which would overflow the pane and
		// scroll the search bar off the top of the alt-screen).
		if m.width > 0 {
			line = ansi.Truncate(line, m.width, "")
		}
		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m TreeModel) renderRow(r Row) string {
	switch r.Kind {
	case RowProject:
		marker := " "
		if r.Active {
			marker = StyleActive.Render("*")
		}
		glyph := "▸"
		if m.expanded[r.Project] {
			glyph = "▾"
		}
		if m.pinned[r.Project] {
			glyph = "📌" + glyph
		}
		return fmt.Sprintf("%s %s %s", glyph, marker, StyleProject.Render(shortProjectName(r.Project)))
	case RowSession:
		// Build the meta (age + msg count + branch) first and reserve its
		// width, then let the title absorb whatever space is left. Rendering
		// the title first (the old behaviour) meant a long title pushed the
		// meta past m.width, where View's final truncation clipped it — so a
		// "397 msgs" session displayed as "· 39". The meta now always survives.
		ago := timeAgo(r.Session.Modified)
		meta := ago
		if r.Session.MsgCount > 0 {
			meta += fmt.Sprintf(" · %d msgs", r.Session.MsgCount)
		}
		branchPlain := ""
		if r.Session.Branch != "" {
			branchPlain = " " + r.Session.Branch
		}

		const indent = 4
		title := r.Session.Title
		if m.width > 0 {
			// indent + space-before-meta(1) + meta + branch must fit; the rest is title.
			avail := m.width - indent - 1 - ansi.StringWidth(meta) - ansi.StringWidth(branchPlain)
			if avail < 0 {
				avail = 0
			}
			if ansi.StringWidth(title) > avail {
				title = ansi.Truncate(title, avail, "…")
			}
		} else if len(title) > 50 {
			// No width known (e.g. tests before SetSize): historical 50-col cap.
			title = title[:47] + "..."
		}

		branch := ""
		if r.Session.Branch != "" {
			branch = " " + StyleBranch.Render(r.Session.Branch)
		}
		return fmt.Sprintf("    %s %s%s",
			StyleSession.Render(title),
			StyleDim.Render(meta),
			branch)
	}
	return ""
}

// SetSize is called by the parent on WindowSizeMsg.
func (m *TreeModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.adjustScroll()
}

// shortProjectName returns the last 1-2 meaningful path segments of a project
// path, with the user's $HOME and a small set of generic mount/container
// segments stripped. The result is purely cosmetic — the full path is still
// used everywhere internally.
//
// Examples (with HOME=/home/alice):
//
//	/home/alice/dev/acme/api          → acme/api
//	/home/alice/sandbox               → sandbox
//	/mnt/c/Users/alice/code/foo       → code/foo
//	/opt/projects/x/y                 → x/y
func shortProjectName(p string) string {
	parts := strings.Split(p, "/")
	skip := projectNameSkipSet()
	out := make([]string, 0, len(parts))
	for _, s := range parts {
		if s == "" || skip[s] {
			continue
		}
		out = append(out, s)
	}
	switch {
	case len(out) >= 2:
		return out[len(out)-2] + "/" + out[len(out)-1]
	case len(out) == 1:
		return out[0]
	default:
		return p
	}
}

// projectNameSkipSet returns segments to drop when shortening a path. It
// derives the user's home segments at call-time and adds generic
// mount-prefix segments commonly seen on Linux/WSL2.
func projectNameSkipSet() map[string]bool {
	skip := map[string]bool{
		// Common Linux mount roots and Windows-side WSL prefixes.
		"home": true, "mnt": true, "Users": true,
		"opt": true, "var": true, "srv": true, "tmp": true,
		// WSL "/mnt/c" + "/mnt/d" drive letters.
		"c": true, "d": true,
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, seg := range strings.Split(home, "/") {
			if seg != "" {
				skip[seg] = true
			}
		}
	}
	return skip
}

func timeAgo(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < 0:
		// Future timestamp (clock skew or test fixture). Don't render
		// "-5m ago"; treat as recent.
		return "just now"
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	}
}
