package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/phr34k/bolt-navi/internal/bolt"
)

func (m Model) View() string {
	if m.width == 0 {
		return "loading…"
	}
	switch m.mode {
	case modeWizard:
		return m.viewWizard()
	case modePicker:
		return m.viewPicker()
	case modeDonate:
		return m.viewDonate()
	default:
		return m.viewSearch()
	}
}

// ------------------------------------------------------------------ search --

func (m Model) viewSearch() string {
	w := m.width
	var b strings.Builder

	shown := m.query
	nameStyle := stBold
	if shown == "" {
		shown = "type to search…"
		nameStyle = stDim
	}
	head := stAccent.Render("/ ") + nameStyle.Render(shown)
	count := stDim.Render(fmt.Sprintf("%d match", len(m.results)))
	b.WriteString(padBetween(head, count, w) + "\n")
	b.WriteString(stDim.Render(strings.Repeat("─", w)) + "\n")

	listH := max(1, m.height-3-6)
	off := 0
	if m.sel >= listH {
		off = m.sel - listH + 1
	}
	for row := 0; row < listH; row++ {
		k := off + row
		if k >= len(m.results) {
			b.WriteString("\n")
			continue
		}
		b.WriteString(m.renderRow(m.results[k], k == m.sel, w) + "\n")
	}

	b.WriteString(stDim.Render(strings.Repeat("─", w)) + "\n")
	b.WriteString(m.renderPreview(w))

	donateW := len([]rune(donateLabel))
	gap := max(0, w-visibleLen(footerHints)-donateW)
	foot := stDim.Render(truncate(footerHints, max(0, w-donateW))) +
		strings.Repeat(" ", gap) +
		donateHyperlink(stAccent.Render(donateLabel))
	line := foot
	if m.status != "" {
		line = m.statusStyle().Render(truncate(" "+m.status+" ", w))
	}
	// Pin the footer to the last visible row by padding with blank lines.
	rowsSoFar := strings.Count(b.String(), "\n")
	blanks := max(0, m.height-2-rowsSoFar)
	b.WriteString(strings.Repeat("\n", 1+blanks))
	b.WriteString(line)
	return b.String()
}

func (m Model) renderRow(r result, selected bool, w int) string {
	badge := " TASK "
	bStyle := stTask
	if r.e.Kind == "plan" {
		badge, bStyle = " PLAN ", stPlan
	}
	name := r.e.Name
	desc := r.e.Desc

	if selected {
		plain := fmt.Sprintf("  %s  %s", badge, name)
		if desc != "" {
			plain += "  " + desc
		}
		return stSel.Render(pad(truncate(plain, w), w))
	}

	// Budget the text after the fixed "  BADGE  " prefix (visible width 10) so
	// the badge keeps its background even when the row overflows -- truncating
	// the name/desc instead of dropping all styling.
	const prefix = 10 // 2 + len(" TASK ") + 2
	rest := max(0, w-prefix)
	nameR := []rune(name)
	if len(nameR) > rest {
		nameR = nameR[:rest]
	}
	nameShown := string(nameR)
	out := "  " + bStyle.Render(badge) + "  " + highlight(nameShown, clampIdxs(r.idxs, len(nameR)))

	remaining := rest - len(nameR)
	if desc != "" && remaining > 2 {
		out += "  " + stDim.Render(truncate(desc, remaining-2))
	}
	return out
}

// clampIdxs drops matched indexes that fall outside a truncated name.
func clampIdxs(idxs []int, n int) []int {
	var out []int
	for _, i := range idxs {
		if i < n {
			out = append(out, i)
		}
	}
	return out
}

func (m Model) renderPreview(w int) string {
	e, ok := m.current()
	if !ok {
		return ""
	}
	var b strings.Builder
	b.WriteString(stOk.Render(e.Name))
	if e.Detail == nil {
		b.WriteString("\n" + stDim.Render("(parameters load on open)"))
		return b.String()
	}
	var req, opt []string
	for _, p := range e.Detail.Params {
		if bolt.IsRequired(e.Kind, p) {
			req = append(req, p.Name)
		} else {
			opt = append(opt, p.Name)
		}
	}
	summ := fmt.Sprintf("%d param(s)", len(e.Detail.Params))
	if len(req) > 0 {
		summ += fmt.Sprintf("  ·  %d required", len(req))
	}
	b.WriteString("   " + stDim.Render(summ) + "\n")
	if len(req) > 0 {
		b.WriteString(stReq.Render("required: ") + stBold.Render(truncate(strings.Join(req, ", "), w-12)) + "\n")
	} else {
		b.WriteString("\n")
	}
	if len(opt) > 0 {
		b.WriteString(stDim.Render("optional: " + truncate(strings.Join(opt, ", "), w-12)))
	}
	return b.String()
}

// ------------------------------------------------------------------ wizard --

func (m Model) viewWizard() string {
	w, h := m.width, m.height

	badge := " TASK "
	bStyle := stTask
	if m.entry.Kind == "plan" {
		badge, bStyle = " PLAN ", stPlan
	}
	nreq := len(bolt.MissingRequired(m.fields))
	tag := " all required set "
	if nreq > 0 {
		tag = fmt.Sprintf(" %d required left ", nreq)
	}

	labelW := 10
	for _, f := range m.fields {
		if f.Widget != "button" && len(f.Name) > labelW {
			labelW = min(18, len(f.Name)+1)
		}
	}

	var lines []string
	lines = append(lines, padBetween(bStyle.Render(badge)+" "+stSel.Render("configure "+m.entry.Name), stReq.Render(tag), w))
	lines = append(lines, stDim.Render(strings.Repeat("─", w)))

	const fieldStart = 2
	listH := max(3, h-11)
	foff := 0
	if m.fi >= listH {
		foff = m.fi - listH + 1
	}
	for row := 0; row < listH; row++ {
		i := foff + row
		if i >= len(m.fields) {
			lines = append(lines, "")
			continue
		}
		if m.fields[i].Widget == "button" && i+1 < len(m.fields) && m.fields[i+1].Widget == "button" {
			lines = append(lines, "")
			lines = append(lines, m.renderButtonPair(i, i+1))
			row++
			continue
		}
		lines = append(lines, m.renderField(i, labelW, w))
	}

	// focused-field help block
	f := m.fields[m.fi]
	lines = append(lines, stDim.Render(strings.Repeat("─", w)))
	if f.Widget != "button" {
		meta := fmt.Sprintf("%s : %s", f.Name, orAny(f.Type))
		if f.Default != "" {
			meta += "   default=" + f.Default
		}
		if f.Required {
			meta += "   (required)"
		}
		lines = append(lines, stBold.Render(truncate(meta, w)))
		help := f.Desc
		if help == "" {
			help = "(no description)"
		}
		lines = append(lines, stDim.Render(truncate(help, w)))
	} else {
		hint := "executes the command shown below"
		if f.Name != "Run" {
			hint = "discard and return to search"
		}
		lines = append(lines, stDim.Render(hint))
		lines = append(lines, "")
	}

	// live command preview
	argv := bolt.BuildArgv(m.entry.Kind, m.entry.Name, m.fields)
	lines = append(lines, stDim.Render(strings.Repeat("─", w)))
	lines = append(lines, stOk.Render("$ "+truncate(bolt.Preview(argv), w-2)))

	// footer / status
	foot := " ↑↓ move · Space menu/targets · Enter next · Ctrl+R run · Ctrl+T shell · Ctrl+C back "
	if m.enumOpen {
		foot = " ↑↓ choose · Enter select · Esc cancel "
	}
	footer := stDim.Render(truncate(foot, w))
	if m.status != "" {
		footer = m.statusStyle().Render(truncate(" "+m.status+" ", w))
	}
	lines = append(lines, footer)

	// float the enum dropdown over the field list
	if m.enumOpen {
		box := m.enumBox()
		indent := labelW + 4
		fieldRow := fieldStart + (m.fi - foff)
		start := fieldRow + 1
		regionEnd := fieldStart + listH - 1
		if start+len(box)-1 > regionEnd { // not enough room below -> flip above
			start = fieldRow - len(box)
			if start < fieldStart {
				start = fieldStart
			}
		}
		overlay(lines, start, indent, box)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderButtonPair(i, j int) string {
	btn := func(idx int) string {
		f := m.fields[idx]
		s := fmt.Sprintf("[  %s  ]", f.Name)
		if idx == m.fi {
			return stSel.Render(s)
		}
		return stBold.Render(s)
	}
	return "    " + btn(i) + "   " + btn(j)
}

func (m Model) renderField(i, labelW, w int) string {
	f := m.fields[i]
	focused := i == m.fi

	if f.Widget == "button" {
		btn := fmt.Sprintf("[  %s  ]", f.Name)
		if focused {
			return "    " + stSel.Render(btn)
		}
		return "    " + stBold.Render(btn)
	}

	mark := " "
	if f.Required {
		mark = stReq.Render("*")
	}
	label := pad(f.Name, labelW)
	if focused {
		label = stAccent.Render(label)
	} else {
		label = stBold.Render(label)
	}
	return fmt.Sprintf(" %s %s %s", mark, label, m.renderValue(f, focused))
}

func (m Model) renderValue(f bolt.Field, focused bool) string {
	switch {
	case f.Widget == "enum":
		cur := ""
		if f.CI < len(f.Choices) {
			cur = f.Choices[f.CI]
		}
		if cur == "" {
			cur = "(unset)"
		}
		s := cur + " ▾"
		if focused {
			return stSel.Render(s)
		}
		return stDim.Render(s)
	case f.Role == "targets" && f.Value == "":
		if m.targetsState == "loading" {
			return stAccent.Render("⠿ loading targets…")
		}
		return stDim.Render("‹Space to pick targets›")
	default:
		val := f.Value
		if f.Sensitive && val != "" {
			val = strings.Repeat("•", len([]rune(val)))
		}
		if val == "" && !focused {
			return stDim.Render("(empty)")
		}
		if focused {
			return val + "▌"
		}
		return val
	}
}

// enumBox builds the floating dropdown for the focused enum field.
func (m Model) enumBox() []string {
	f := m.fields[m.fi]
	labels := make([]string, len(f.Choices))
	innerW := len("choose") + 2
	for i, c := range f.Choices {
		l := c
		if l == "" {
			l = "(unset)"
		}
		labels[i] = l
		if n := len([]rune(l)) + 2; n > innerW {
			innerW = n
		}
	}
	if innerW > 32 {
		innerW = 32
	}
	rows := make([]string, len(labels))
	for i, l := range labels {
		if i == m.enumIdx {
			rows[i] = stSel.Render(pad("› "+l, innerW))
		} else {
			rows[i] = pad("  "+l, innerW)
		}
	}
	return boxLines("choose", "", rows, innerW)
}

// ------------------------------------------------------------------ donate dialog --

func (m Model) viewDonate() string {
	w, h := m.width, m.height
	innerW := min(56, max(44, w-8))

	const (
		btnOpenText    = "[  Open donation page  ]" // 24 visible chars
		btnDismissText = "[  Dismiss  ]"             // 13 visible chars
	)
	var btnOpen, btnDismiss string
	if m.donateBtnIdx == 0 {
		btnOpen, btnDismiss = stSel.Render(btnOpenText), stBold.Render(btnDismissText)
	} else {
		btnOpen, btnDismiss = stBold.Render(btnOpenText), stSel.Render(btnDismissText)
	}
	// Visible width of button pair: 2 + 24 + 3 + 13 = 42; pad the remainder.
	btnTrail := strings.Repeat(" ", max(0, innerW-42))
	btnRow := "  " + btnOpen + "   " + btnDismiss + btnTrail

	rows := []string{
		pad("", innerW),
		pad("  bolt-navi is free & open-source.", innerW),
		pad("  If you find it useful, please consider", innerW),
		pad("  supporting the project — thank you!", innerW),
		pad("", innerW),
		btnRow,
		pad("", innerW),
		stDim.Render(pad("  ←→ navigate · Enter confirm · Esc dismiss", innerW)),
		pad("", innerW),
	}
	box := boxLines("Support bolt-navi", "", rows, innerW)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, strings.Join(box, "\n"))
}

// ------------------------------------------------------------------ picker --

func (m Model) viewPicker() string {
	w, h := m.width, m.height
	innerW := min(64, max(28, w-8))

	type vrow struct {
		text    string
		itemIdx int // -1 for a section header
	}
	var vrows []vrow
	lastGroup := -1
	for i, it := range m.pickItems {
		g := 0
		if it.group {
			g = 1
		}
		if g != lastGroup {
			hdr := "TARGETS"
			if it.group {
				hdr = "GROUPS"
			}
			vrows = append(vrows, vrow{stDim.Render(pad(hdr, innerW)), -1})
			lastGroup = g
		}
		glyph := "○"
		if m.pickChecked[i] {
			glyph = "●"
		}
		raw := pad(fmt.Sprintf(" %s %s", glyph, it.val), innerW)
		var styled string
		switch {
		case i == m.pickIdx:
			styled = stSel.Render(raw)
		case m.pickChecked[i]:
			styled = stBold.Render(raw)
		default:
			styled = stDim.Render(raw)
		}
		vrows = append(vrows, vrow{styled, i})
	}

	// window so the highlighted item stays visible, leaving room for a footer
	maxRows := max(3, h-7)
	selRow := 0
	for r, vr := range vrows {
		if vr.itemIdx == m.pickIdx {
			selRow = r
			break
		}
	}
	off := 0
	if selRow >= maxRows {
		off = selRow - maxRows + 1
	}
	end := min(len(vrows), off+maxRows)

	var rows []string
	for _, vr := range vrows[off:end] {
		rows = append(rows, vr.text)
	}
	rows = append(rows, pad("", innerW))
	rows = append(rows, stDim.Render(pad("Space toggle · a all · n none · Enter ok · Esc cancel", innerW)))

	n := 0
	for _, c := range m.pickChecked {
		if c {
			n++
		}
	}
	box := boxLines("Select targets", fmt.Sprintf("%d selected", n), rows, innerW)
	return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, strings.Join(box, "\n"))
}

// ------------------------------------------------------------------ boxes --

// boxLines frames content rows (each already padded to innerW) in a single-line
// border, embedding an optional left title and right-aligned label into the top
// edge -- matching the Python box() look.
func boxLines(title, right string, rows []string, innerW int) []string {
	mid := []rune(strings.Repeat("─", innerW+2))
	if title != "" {
		embed(mid, " "+title+" ", 1)
	}
	if right != "" {
		r := []rune(" " + right + " ")
		embed(mid, string(r), len(mid)-1-len(r))
	}
	v := stAccent.Render("│")
	out := []string{stAccent.Render("╭" + string(mid) + "╮")}
	for _, r := range rows {
		out = append(out, v+" "+r+" "+v)
	}
	out = append(out, stAccent.Render("╰"+strings.Repeat("─", innerW+2)+"╯"))
	return out
}

// embed writes s into dst starting at col, staying inside the border corners.
func embed(dst []rune, s string, col int) {
	for i, ch := range []rune(s) {
		p := col + i
		if p > 0 && p < len(dst)-1 {
			dst[p] = ch
		}
	}
}

// overlay floats box rows over lines at (start, indent), overwriting only the
// box's column span so the base content to the left (field labels) and right
// stays visible -- matching the Python popup, which cleared only behind itself.
func overlay(lines []string, start, indent int, box []string) {
	if len(box) == 0 {
		return
	}
	boxW := visibleLen(box[0])
	for i, br := range box {
		r := start + i
		if r < 0 || r >= len(lines) {
			continue
		}
		base := lines[r]
		left := sliceVisible(base, 0, indent)
		if lw := visibleLen(left); lw < indent {
			left += strings.Repeat(" ", indent-lw)
		}
		right := sliceVisible(base, indent+boxW, 1<<30)
		lines[r] = left + "\x1b[0m" + br + right
	}
}

// sliceVisible returns the portion of s covering visible columns [from, to),
// keeping any ANSI escape sequences that occur within that span.
func sliceVisible(s string, from, to int) string {
	var b strings.Builder
	rs := []rune(s)
	col, i := 0, 0
	for i < len(rs) {
		if rs[i] == 0x1b { // copy a whole \x1b[…m escape, counts as zero width
			j := i
			for j < len(rs) && rs[j] != 'm' {
				j++
			}
			if j < len(rs) {
				j++
			}
			if col >= from && col < to {
				b.WriteString(string(rs[i:j]))
			}
			i = j
			continue
		}
		if col >= from && col < to {
			b.WriteRune(rs[i])
		}
		col++
		i++
	}
	return b.String()
}

// ------------------------------------------------------------------ text utils --

func (m Model) statusStyle() lipgloss.Style {
	if m.statusErr {
		return stErr
	}
	return stOk
}

// highlight renders name with the fuzzy-matched rune positions in the accent
// color and the rest bold.
func highlight(name string, idxs []int) string {
	if len(idxs) == 0 {
		return stBold.Render(name)
	}
	hit := make(map[int]bool, len(idxs))
	for _, i := range idxs {
		hit[i] = true
	}
	var b strings.Builder
	for i, ch := range []rune(name) {
		if hit[i] {
			b.WriteString(stAccent.Render(string(ch)))
		} else {
			b.WriteString(stBold.Render(string(ch)))
		}
	}
	return b.String()
}

func orAny(s string) string {
	if s == "" {
		return "Any"
	}
	return s
}

func pad(s string, w int) string {
	r := []rune(s)
	if len(r) >= w {
		return string(r[:w])
	}
	return s + strings.Repeat(" ", w-len(r))
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:w])
}

// padBetween puts left and right on one line, filling the gap, truncating the
// left side if the two don't fit. Widths are measured ignoring ANSI escapes.
func padBetween(left, right string, w int) string {
	lw := visibleLen(left)
	rw := visibleLen(right)
	gap := w - lw - rw
	if gap < 1 {
		return truncate(left, max(0, w-rw)) + right
	}
	return left + strings.Repeat(" ", gap) + right
}

// visibleLen counts runes outside ANSI escape sequences.
func visibleLen(s string) int {
	n, inEsc := 0, false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case inEsc:
		default:
			n++
		}
	}
	return n
}
