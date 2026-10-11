// Package ui is the Bubble Tea front-end for bolt-tui. It is a direct port of
// the Python "hybrid" concept: a search-first palette that opens into a
// single-page run wizard, with an inline target picker and a shell escape
// hatch. All data/exec logic lives in internal/bolt; this file is layout,
// flow, and keybindings only.
package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/phr34k/bolt-navi/internal/bolt"
)

type mode int

const (
	modeSearch mode = iota
	modeWizard
	modePicker
	modeDonate
)

type result struct {
	e    bolt.Entry
	idxs []int // fuzzy-matched rune indexes in the name
}

// Model is the whole application state.
type Model struct {
	root         string
	all          []bolt.Entry
	initTargets  string // pre-populated from --targets flag

	width, height int

	// search
	query   string
	results []result
	sel     int

	// wizard
	mode     mode
	entry    bolt.Entry
	fields   []bolt.Field
	fi       int
	foff     int
	enumOpen bool
	enumIdx  int

	// targets
	names, groups []string
	targetsState  string // "loading" | "ready"
	wantPicker    bool

	// picker
	pickItems   []pickItem
	pickChecked map[int]bool
	pickIdx     int

	// donate dialog
	donateBtnIdx int // 0 = open page, 1 = dismiss

	status    string
	statusErr bool
}

type pickItem struct {
	group bool
	val   string
}

// messages
type targetsMsg struct{ names, groups []string }
type moduleInstallDoneMsg struct{ err error }
type clearStatusMsg struct{}
type shellDoneMsg struct {
	dump    string
	cleanup func()
}
type runDoneMsg struct {
	name string
	code int
}

// New builds the initial model. targets is an optional comma-separated list of
// targets that will pre-populate the targets field whenever the wizard opens.
func New(root, targets string) Model {
	startMode := modeSearch
	if shouldShowDonateDialog(incrementStartupCount()) {
		startMode = modeDonate
	}
	m := Model{
		root:         root,
		all:          bolt.LoadAll(root),
		mode:         startMode,
		initTargets:  targets,
		targetsState: "loading",
		pickChecked:  map[int]bool{},
		donateBtnIdx: 1, // default cursor on Dismiss so Enter never surprises
	}
	m.refilter()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadTargetsCmd(), m.moduleInstallCmd())
}

func (m Model) loadTargetsCmd() tea.Cmd {
	root := m.root
	return func() tea.Msg {
		n, g := bolt.LoadTargetsFast(root)
		return targetsMsg{n, g}
	}
}

func (m Model) moduleInstallCmd() tea.Cmd {
	root := m.root
	return func() tea.Msg {
		return moduleInstallDoneMsg{bolt.ModuleInstall(root)}
	}
}

// ------------------------------------------------------------------ Update --

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case targetsMsg:
		m.names, m.groups = msg.names, msg.groups
		m.targetsState = "ready"
		if m.wantPicker && m.mode == modeWizard && m.currentField().Role == "targets" {
			m.openPicker()
		}
		m.wantPicker = false
		return m, nil

	case moduleInstallDoneMsg:
		if msg.err == nil {
			m.all = bolt.LoadAll(m.root)
			m.refilter()
		}
		return m, nil

	case shellDoneMsg:
		n := bolt.MergeEnvDump(msg.dump)
		if msg.cleanup != nil {
			msg.cleanup()
		}
		if n > 0 {
			m.setStatus(fmt.Sprintf("back from shell · %d env var(s) updated", n), false)
		} else {
			m.setStatus("back from shell", false)
		}
		return m, clearStatusAfter(3 * time.Second)

	case runDoneMsg:
		m.mode = modeSearch
		m.setStatus(fmt.Sprintf("last run of %s exited %d", msg.name, msg.code), msg.code != 0)
		return m, nil

	case clearStatusMsg:
		m.status = ""
		return m, nil

	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft {
			cmd := m.handleMouseClick(msg.X, msg.Y)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		var cmd tea.Cmd
		switch m.mode {
		case modeSearch:
			cmd = m.handleSearchKey(msg)
		case modeWizard:
			cmd = m.handleWizardKey(msg)
		case modePicker:
			m.handlePickerKey(msg)
		case modeDonate:
			cmd = m.handleDonateKey(msg)
		}
		return m, cmd
	}
	return m, nil
}


// ------------------------------------------------------------------ search --

func (m *Model) refilter() {
	q := strings.TrimSpace(m.query)
	type scored struct {
		score float64
		e     bolt.Entry
		idxs  []int
	}
	var ss []scored
	for _, e := range m.all {
		if q == "" {
			ss = append(ss, scored{0, e, nil})
			continue
		}
		if mm, ok := bolt.Fuzzy(q, e.Name); ok {
			ss = append(ss, scored{mm.Score, e, mm.Indexes})
		} else if e.Desc != "" {
			if dm, ok := bolt.Fuzzy(q, e.Desc); ok {
				ss = append(ss, scored{dm.Score - 100, e, nil})
			}
		}
	}
	if q != "" {
		sort.SliceStable(ss, func(i, j int) bool {
			if ss[i].score != ss[j].score {
				return ss[i].score > ss[j].score
			}
			im, ir := bolt.ModuleSortKey(ss[i].e.Name)
			jm, jr := bolt.ModuleSortKey(ss[j].e.Name)
			if im != jm {
				return im < jm
			}
			return ir < jr
		})
	}
	m.results = make([]result, len(ss))
	for i, s := range ss {
		m.results[i] = result{s.e, s.idxs}
	}
	if m.sel > len(m.results)-1 {
		m.sel = max(0, len(m.results)-1)
	}
}

func (m *Model) current() (bolt.Entry, bool) {
	if len(m.results) == 0 {
		return bolt.Entry{}, false
	}
	return m.results[min(m.sel, len(m.results)-1)].e, true
}

func (m *Model) handleSearchKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "ctrl+c", "esc":
		if m.query != "" {
			m.query, m.sel = "", 0
			m.refilter()
			return nil
		}
		return tea.Quit
	case "ctrl+t":
		return m.openShellCmd()
	case "ctrl+d":
		if err := openBrowser(donateURL); err != nil {
			m.setStatus("could not open browser", true)
		} else {
			m.setStatus("opening donation page…", false)
		}
		return clearStatusAfter(3 * time.Second)
	case "up":
		m.sel = max(0, m.sel-1)
	case "down":
		m.sel = min(len(m.results)-1, m.sel+1)
	case "pgup":
		m.sel = max(0, m.sel-10)
	case "pgdown":
		m.sel = min(len(m.results)-1, m.sel+10)
	case "enter":
		m.openWizard()
	case "backspace":
		if r := []rune(m.query); len(r) > 0 {
			m.query = string(r[:len(r)-1])
			m.sel = 0
			m.refilter()
		}
	default:
		if typed := typable(msg); typed != "" {
			m.query += typed
			m.sel = 0
			m.refilter()
		}
	}
	return nil
}

// typable returns the printable characters of a key message, dropping control
// runes (e.g. the NUL that Ctrl+Space emits on Windows) that would otherwise
// corrupt a field value and later break process creation.
func typable(msg tea.KeyMsg) string {
	if msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace {
		return ""
	}
	var b strings.Builder
	for _, r := range msg.String() {
		if r >= 0x20 && r != 0x7f {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ------------------------------------------------------------------ wizard --

func (m *Model) currentField() bolt.Field {
	if m.fi < len(m.fields) {
		return m.fields[m.fi]
	}
	return bolt.Field{}
}

func (m *Model) openWizard() {
	e, ok := m.current()
	if !ok {
		return
	}
	if err := bolt.EnsureDetail(m.root, &e); err != nil {
		m.setStatus(fmt.Sprintf("cannot load %s: %v", e.Name, err), true)
		return
	}
	m.entry = e
	m.fields = bolt.BuildFields(e.Kind, e.Detail)
	if m.initTargets != "" {
		for i := range m.fields {
			if m.fields[i].Role == "targets" && m.fields[i].Value == "" {
				m.fields[i].Value = m.initTargets
				break
			}
		}
	}
	m.fields = append(m.fields,
		bolt.Field{Name: "Run", Role: "button", Widget: "button"},
		bolt.Field{Name: "Cancel", Role: "button", Widget: "button"})
	m.fi, m.foff = 0, 0
	m.enumOpen = false
	m.mode = modeWizard
	m.status = ""
}

func (m *Model) handleWizardKey(msg tea.KeyMsg) tea.Cmd {
	if m.enumOpen {
		m.handleEnumKey(msg)
		return nil
	}
	f := &m.fields[m.fi]
	switch msg.String() {
	case "ctrl+t":
		return m.openShellCmd()
	case "ctrl+r":
		return m.runWizard()
	case "ctrl+c", "esc":
		m.wantPicker = false
		m.mode = modeSearch
		m.status = ""
		return nil
	case "down", "tab":
		m.wantPicker = false
		m.fi = (m.fi + 1) % len(m.fields)
		return nil
	case "up", "shift+tab":
		m.wantPicker = false
		m.fi = (m.fi - 1 + len(m.fields)) % len(m.fields)
		return nil
	}

	switch f.Widget {
	case "button":
		switch msg.String() {
		case "enter", " ":
			if f.Name == "Run" {
				return m.runWizard()
			}
			m.mode = modeSearch
			m.status = ""
		case "left":
			if prev := (m.fi - 1 + len(m.fields)) % len(m.fields); m.fields[prev].Widget == "button" {
				m.fi = prev
			}
		case "right":
			if next := (m.fi + 1) % len(m.fields); m.fields[next].Widget == "button" {
				m.fi = next
			}
		}
	case "enum":
		switch msg.String() {
		case " ":
			m.enumOpen = true
			m.enumIdx = f.CI
		case "enter":
			m.fi = (m.fi + 1) % len(m.fields)
		}
	default: // text / targets
		if f.Role == "targets" {
			switch msg.String() {
			case "enter":
				m.fi = (m.fi + 1) % len(m.fields)
				return nil
			case " ":
				if m.targetsState == "ready" {
					m.openPicker()
				} else {
					m.wantPicker = true
					m.setStatus("targets still loading…", false)
				}
				return nil
			}
		}
		m.editText(f, msg)
	}
	return nil
}

func (m *Model) editText(f *bolt.Field, msg tea.KeyMsg) {
	switch msg.String() {
	case "enter":
		m.fi = (m.fi + 1) % len(m.fields)
	case "backspace":
		if r := []rune(f.Value); len(r) > 0 {
			f.Value = string(r[:len(r)-1])
		}
	default:
		f.Value += typable(msg)
	}
}

func (m *Model) handleEnumKey(msg tea.KeyMsg) {
	f := &m.fields[m.fi]
	n := len(f.Choices)
	if n == 0 {
		m.enumOpen = false
		return
	}
	switch msg.String() {
	case "up", "shift+tab":
		m.enumIdx = (m.enumIdx - 1 + n) % n
	case "down", "tab":
		m.enumIdx = (m.enumIdx + 1) % n
	case "enter", " ":
		f.CI = m.enumIdx
		f.Value = f.Choices[f.CI]
		m.enumOpen = false
	case "esc", "ctrl+c":
		m.enumOpen = false
	}
}

func (m *Model) runWizard() tea.Cmd {
	if miss := bolt.MissingRequired(m.fields); len(miss) > 0 {
		m.setStatus("missing required: "+strings.Join(miss, ", "), true)
		for i, f := range m.fields {
			if contains(miss, f.Name) {
				m.fi = i
				break
			}
		}
		return nil
	}
	argv := bolt.BuildArgv(m.entry.Kind, m.entry.Name, m.fields)
	return runExec(m.root, m.entry.Name, argv, bolt.Preview(argv))
}

func (m *Model) openShellCmd() tea.Cmd {
	cmd, dump, cleanup, err := bolt.ShellCmd()
	if err != nil {
		m.setStatus(fmt.Sprintf("cannot open shell: %v", err), true)
		return nil
	}
	return tea.ExecProcess(cmd, func(error) tea.Msg {
		return shellDoneMsg{dump: dump, cleanup: cleanup}
	})
}

// ------------------------------------------------------------------ picker --

func (m *Model) openPicker() {
	m.pickItems = m.pickItems[:0]
	for _, g := range m.groups {
		m.pickItems = append(m.pickItems, pickItem{group: true, val: g})
	}
	for _, t := range m.names {
		m.pickItems = append(m.pickItems, pickItem{group: false, val: t})
	}
	if len(m.pickItems) == 0 {
		m.setStatus("no targets found in inventory", true)
		return
	}
	m.pickChecked = map[int]bool{}
	m.pickIdx = 0
	m.mode = modePicker
}

func (m *Model) handlePickerKey(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		m.pickIdx = max(0, m.pickIdx-1)
	case "down", "j":
		m.pickIdx = min(len(m.pickItems)-1, m.pickIdx+1)
	case "home":
		m.pickIdx = 0
	case "end":
		m.pickIdx = len(m.pickItems) - 1
	case " ":
		m.pickChecked[m.pickIdx] = !m.pickChecked[m.pickIdx]
	case "a":
		for i := range m.pickItems {
			m.pickChecked[i] = true
		}
	case "n":
		m.pickChecked = map[int]bool{}
	case "enter":
		var picked []string
		for i, it := range m.pickItems {
			if m.pickChecked[i] {
				picked = append(picked, it.val)
			}
		}
		if m.fi < len(m.fields) {
			m.fields[m.fi].Value = strings.Join(picked, ",")
		}
		m.mode = modeWizard
	case "esc", "ctrl+c":
		m.mode = modeWizard
	}
}

func (m *Model) handleMouseClick(x, y int) tea.Cmd {
	if m.mode != modeSearch || m.status != "" {
		return nil
	}
	donateW := len([]rune(donateLabel))
	donateCol := m.width - donateW
	if y == m.height-1 && x >= donateCol && x < m.width {
		if err := openBrowser(donateURL); err != nil {
			m.setStatus("could not open browser", true)
		} else {
			m.setStatus("opening donation page…", false)
		}
		return clearStatusAfter(3 * time.Second)
	}
	return nil
}

func clearStatusAfter(d time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(d)
		return clearStatusMsg{}
	}
}

// ------------------------------------------------------------------ donate dialog --

func (m *Model) handleDonateKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "left", "right", "tab", "shift+tab":
		m.donateBtnIdx = 1 - m.donateBtnIdx
	case "enter", " ":
		m.mode = modeSearch
		if m.donateBtnIdx == 0 {
			if err := openBrowser(donateURL); err != nil {
				m.setStatus("could not open browser", true)
			} else {
				m.setStatus("opening donation page…", false)
			}
			return clearStatusAfter(3 * time.Second)
		}
	case "esc", "ctrl+c":
		m.mode = modeSearch
	}
	return nil
}

// ------------------------------------------------------------------ helpers --

func (m *Model) setStatus(s string, isErr bool) {
	m.status, m.statusErr = s, isErr
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
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

// styles
var (
	stAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true)
	stDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	stTask   = lipgloss.NewStyle().Background(lipgloss.Color("6")).Foreground(lipgloss.Color("0")).Bold(true)
	stPlan   = lipgloss.NewStyle().Background(lipgloss.Color("5")).Foreground(lipgloss.Color("0")).Bold(true)
	stReq    = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true)
	stOk     = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true)
	stErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)
	stSel    = lipgloss.NewStyle().Background(lipgloss.Color("5")).Foreground(lipgloss.Color("0")).Bold(true)
	stBold   = lipgloss.NewStyle().Bold(true)
)
