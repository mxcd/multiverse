package tui

import (
	"fmt"
	"strings"

	"github.com/mxcd/multiverse/internal/brain"
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.tabBar())
	b.WriteString("\n\n")

	switch m.view {
	case dashView:
		b.WriteString(m.viewDashboard())
	case brainsView:
		b.WriteString(m.viewBrains())
	case scopeView:
		b.WriteString(m.viewScope())
	}

	b.WriteString("\n")
	b.WriteString(m.bar())
	return b.String()
}

func (m Model) tabBar() string {
	tab := func(id viewID, label string) string {
		if m.view == id {
			return tabActive.Render(label)
		}
		return tabInactive.Render(label)
	}
	title := titleStyle.Render("multi")
	return title + "  " + tab(dashView, "Dashboard") + tab(brainsView, "Brains") + tab(scopeView, "Scope")
}

func (m Model) viewDashboard() string {
	if len(m.rows) == 0 {
		return dimStyle.Render("no brains registered — go to Brains (tab) and press 'a', or run `multi init`/`multi clone`")
	}
	var b strings.Builder
	b.WriteString(headStyle.Render(fmt.Sprintf("  %-16s %-12s %-18s %6s  %s", "BRAIN", "BRANCH", "GIT", "NOTES", "LINT")))
	b.WriteString("\n")
	for i, r := range m.rows {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▌ ")
		}
		mark := " "
		if r.active {
			mark = activeMark.Render("●")
		}
		name := fmt.Sprintf("%-16s", truncate(r.name, 16))
		branch := dimStyle.Render(fmt.Sprintf("%-12s", truncate(r.info.Branch, 12)))
		notes := fmt.Sprintf("%6d", r.notes)
		b.WriteString(fmt.Sprintf("%s%s %s %s %s  %s  %s\n",
			cursor, mark, name, branch, m.gitCell(r), notes, lintCell(r.lint)))
	}
	b.WriteString("\n")
	b.WriteString(m.scopeSummary())
	return b.String()
}

func (m Model) gitCell(r brainRow) string {
	if r.loading {
		return dimStyle.Render(fmt.Sprintf("%-18s", "loading…"))
	}
	if r.err != "" {
		return errStyle.Render(fmt.Sprintf("%-18s", "error"))
	}
	if !r.info.IsGit {
		return warnStyle.Render(fmt.Sprintf("%-18s", "not a git repo"))
	}
	state := okStyle.Render("clean")
	if r.info.Dirty {
		state = warnStyle.Render("dirty")
	}
	ab := ""
	if r.info.HasUpstream {
		if r.info.Ahead > 0 || r.info.Behind > 0 {
			ab = accentStyle.Render(fmt.Sprintf(" ↑%d ↓%d", r.info.Ahead, r.info.Behind))
		}
	} else if r.info.HasRemote {
		ab = dimStyle.Render(" no-upstream")
	} else {
		ab = dimStyle.Render(" local")
	}
	// pad on the visible (uncolored) length
	plain := stripState(r.info)
	pad := 18 - len(plain)
	if pad < 0 {
		pad = 0
	}
	return state + ab + strings.Repeat(" ", pad)
}

func stripState(info brain.GitInfo) string {
	s := "clean"
	if info.Dirty {
		s = "dirty"
	}
	switch {
	case info.HasUpstream && (info.Ahead > 0 || info.Behind > 0):
		s += fmt.Sprintf(" ↑%d ↓%d", info.Ahead, info.Behind)
	case info.HasUpstream:
	case info.HasRemote:
		s += " no-upstream"
	default:
		s += " local"
	}
	return s
}

func lintCell(v string) string {
	switch {
	case v == "":
		return dimStyle.Render("—")
	case v == "ok":
		return okStyle.Render("ok")
	case v == "…":
		return dimStyle.Render("…")
	case v == "err":
		return errStyle.Render("err")
	default:
		return errStyle.Render(v)
	}
}

func (m Model) scopeSummary() string {
	var sources, targets []string
	for _, r := range m.rows {
		if m.srcSel[r.name] {
			sources = append(sources, r.name)
		}
		if m.tgtSel[r.name] {
			targets = append(targets, r.name)
		}
	}
	src := dimStyle.Render("(none)")
	if len(sources) > 0 {
		src = strings.Join(sources, ", ")
	}
	tgt := dimStyle.Render("(none)")
	if len(targets) > 0 {
		tgt = strings.Join(targets, ", ")
	}
	return headStyle.Render("SCOPE ") + dimStyle.Render("("+m.cwd+")") + "\n" +
		fmt.Sprintf("  sources: %s\n  targets: %s\n  origin:  %s", src, tgt, dimStyle.Render(m.bindingOrigin))
}

func (m Model) viewBrains() string {
	if len(m.rows) == 0 {
		return dimStyle.Render("no brains registered — press 'a' to add one")
	}
	var b strings.Builder
	b.WriteString(headStyle.Render(fmt.Sprintf("  %-16s %s", "BRAIN", "PATH")))
	b.WriteString("\n")
	for i, r := range m.rows {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▌ ")
		}
		mark := " "
		if r.active {
			mark = activeMark.Render("●")
		}
		b.WriteString(fmt.Sprintf("%s%s %-16s %s\n", cursor, mark, truncate(r.name, 16), dimStyle.Render(r.path)))
	}
	return b.String()
}

func (m Model) viewScope() string {
	var b strings.Builder
	b.WriteString(dimStyle.Render("binding for " + m.cwd))
	b.WriteString("\n\n")
	if len(m.rows) == 0 {
		b.WriteString(dimStyle.Render("no brains to bind — add some under Brains"))
		return b.String()
	}
	b.WriteString(headStyle.Render(fmt.Sprintf("  %-8s %-8s %s", "SOURCE", "TARGET", "BRAIN")))
	b.WriteString("\n")
	for i, r := range m.rows {
		cursor := "  "
		if i == m.cursor {
			cursor = cursorStyle.Render("▌ ")
		}
		b.WriteString(fmt.Sprintf("%s%-8s %-8s %s\n", cursor, checkbox(m.srcSel[r.name]), checkbox(m.tgtSel[r.name]), r.name))
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("space = toggle source · t = toggle target · w = save to ./.multi.yaml · c = clear"))
	return b.String()
}

func checkbox(on bool) string {
	if on {
		return okStyle.Render("[x]")
	}
	return dimStyle.Render("[ ]")
}

func (m Model) bar() string {
	if m.mode != modeNormal {
		return m.inputBar()
	}
	var keys string
	switch m.view {
	case dashView:
		keys = "↑/↓ move · s sync · S sync all · l lint · L lint all · r refresh · enter activate · tab views · q quit"
	case brainsView:
		keys = "↑/↓ move · a add · e rename · p path · d delete · enter activate · tab views · q quit"
	case scopeView:
		keys = "↑/↓ move · space source · t target · w save · c clear · tab views · q quit"
	}
	out := footerStyle.Render(keys)
	if m.status != "" {
		out += "\n" + statusStyle.Render(m.status)
	}
	return out
}

func (m Model) inputBar() string {
	if m.mode == modeConfirmDelete {
		name := ""
		if r, ok := m.current(); ok {
			name = r.name
		}
		return warnStyle.Render(fmt.Sprintf("delete brain %q from the registry? (y/n)", name))
	}
	label := map[inputMode]string{
		modeAddName: "new brain name:",
		modeAddPath: "path to brain directory:",
		modeRename:  "rename to:",
		modeSetPath: "set path:",
	}[m.mode]
	return accentStyle.Render(label) + "\n" + m.input.View() + "\n" + dimStyle.Render("enter = confirm · esc = cancel")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
