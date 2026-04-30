package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
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
		// rebuild order to drop empty projects
		newOrder := order[:0]
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
func (m *TreeModel) SetFilter(filter string) { m.filter = filter; m.cursor = 0 }

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
	}
	return m, nil
}

// SelectedRow returns the currently-cursored row, or zero-value Row if none.
func (m TreeModel) SelectedRow() Row {
	rows := m.flatten(m.filter)
	if m.cursor < 0 || m.cursor >= len(rows) {
		return Row{}
	}
	return rows[m.cursor]
}

// View renders the tree (clipped to width/height set by parent).
func (m TreeModel) View() string {
	rows := m.flatten(m.filter)
	var b strings.Builder
	for i, r := range rows {
		line := m.renderRow(r)
		if i == m.cursor {
			line = StyleSelected.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
		if b.Len() > m.height*m.width*4 { // safety clamp
			break
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
		title := r.Session.Title
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		ago := timeAgo(r.Session.Modified)
		msg := ""
		if r.Session.MsgCount > 0 {
			msg = fmt.Sprintf(" · %d msgs", r.Session.MsgCount)
		}
		branch := ""
		if r.Session.Branch != "" {
			branch = " " + StyleBranch.Render(r.Session.Branch)
		}
		return fmt.Sprintf("    %s %s%s%s",
			StyleSession.Render(title),
			StyleDim.Render(ago),
			StyleDim.Render(msg),
			branch)
	}
	return ""
}

// SetSize is called by the parent on WindowSizeMsg.
func (m *TreeModel) SetSize(w, h int) { m.width = w; m.height = h }

// shortProjectName returns the last 1-2 meaningful path segments.
func shortProjectName(p string) string {
	parts := strings.Split(p, "/")
	skip := map[string]bool{"home": true, "michael": true, "dev": true, "mnt": true, "c": true,
		"PrivateData": true, "Work": true, "www": true, "Users": true, "micha": true, "": true}
	out := []string{}
	for _, s := range parts {
		if skip[s] {
			continue
		}
		out = append(out, s)
	}
	if len(out) >= 2 {
		return out[len(out)-2] + "/" + out[len(out)-1]
	}
	if len(out) == 1 {
		return out[0]
	}
	return p
}

func timeAgo(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
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
