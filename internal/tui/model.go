package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kontrolplane/past/internal/gh"
	"github.com/kontrolplane/past/internal/git"
	"github.com/kontrolplane/past/internal/pass"
	"github.com/kontrolplane/past/internal/theme"
)

type mode int

const (
	modeBrowse mode = iota
	modeCreate      // pick a preset type, then fill its fields in-app
	modeConfirmDelete
	modeGithub
	modeUnlock // enter the gpg passphrase in-app
	modePrompt // type a destination path for rename / duplicate
	modeCommit // type a commit message for the staged changes
	modeCopy   // pick which field of the selected entry to copy
)

// ghView is the active panel inside the github modal.
type ghView int

const (
	ghMenu ghView = iota
	ghAlerts
	ghRuns
)

// Model is the root bubble tea model.
type Model struct {
	tree    tree
	spinner spinner.Model

	showHelp bool
	mode     mode
	width    int
	height   int
	treeH    int // rows allotted to the tree

	current   string // selected entry name backing the detail pane
	detailRaw string // raw decrypted content for the detail pane
	masked    bool

	copyCursor int // selected field in the copy picker (modeCopy)

	status  git.SyncStatus
	repo    gh.Repo
	hasRepo bool

	// create flow: pick a preset, then fill its fields in-app.
	genPicking bool              // choosing a type vs. filling the form
	genPreset  int               // index into presets
	genInputs  []textinput.Model // one per editable field (preset.editable order)
	genLen     textinput.Model   // password length, when the preset generates one
	genFocus   int               // focused control: [inputs..., length?]
	symbols    bool

	passInput textinput.Model // gpg passphrase prompt (masked)

	// rename / duplicate prompt: edit a destination path for promptSrc.
	promptInput textinput.Model
	promptSrc   string
	promptCopy  bool // true = duplicate (cp), false = rename/move (mv)

	// github modal state
	ghView   ghView
	ghCursor int
	alerts   []gh.Alert
	runs     []gh.Run

	ready     bool // startup auth probe resolved — safe to render the store
	busy      bool
	message   string
	messageAt time.Time // when message was set, for auto-clear
	isErr     bool
}

// New constructs the initial model.
func New() Model {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = theme.Accent

	length := textinput.New()
	length.Placeholder = "24"
	length.CharLimit = 3

	pp := textinput.New()
	pp.Placeholder = "passphrase"
	pp.EchoMode = textinput.EchoPassword
	pp.EchoCharacter = '•'

	prompt := textinput.New()
	prompt.Placeholder = "folder/name"

	return Model{
		tree:        newTree(),
		spinner:     sp,
		masked:      true,
		genLen:      length,
		symbols:     true,
		passInput:   pp,
		promptInput: prompt,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadEntries, loadStatus, checkAuth, m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		// ponytail: piggyback the always-running spinner tick to expire flashes,
		// rather than adding a second ticker just for this.
		if m.message != "" && time.Since(m.messageAt) > 5*time.Second {
			m.message = ""
		}
		return m, cmd

	case entriesMsg:
		m.tree.setEntries([]string(msg))
		return m, m.selectCurrent()

	case clearDetailMsg:
		m.current = ""
		m.detailRaw = ""
		m.renderDetail()
		return m, nil

	case detailMsg:
		m.current = msg.name
		m.detailRaw = msg.content
		m.masked = true
		m.renderDetail()
		return m, nil

	case statusMsg:
		m.status = git.SyncStatus(msg)
		m.hasRepo = m.status.HasRepo
		if r, ok := gh.OriginRepo(); ok {
			m.repo = r
		}
		return m, nil

	case copiedMsg:
		text := "copied " + msg.name
		m.flash(text, false)
		return m, nil

	case syncDoneMsg:
		m.busy = false
		m.flash("synced", false)
		return m, loadStatus

	case provDoneMsg:
		m.busy = false
		m.flash("provisioned "+msg.name, false)
		return m, loadStatus

	case alertsMsg:
		m.busy = false
		m.alerts = msg
		m.ghView = ghAlerts
		return m, nil

	case runsMsg:
		m.busy = false
		m.runs = msg
		m.ghView = ghRuns
		return m, nil

	case editDoneMsg:
		if msg.err != nil {
			m.flash(msg.err.Error(), true)
		}
		return m, tea.Batch(recordEdit(m.current), loadEntries)

	case changedMsg:
		return m, tea.Batch(loadEntries, loadStatus)

	case committedMsg:
		m.flash("committed", false)
		return m, loadStatus

	case authOKMsg:
		m.ready = true
		return m, nil

	case movedMsg:
		m.flash("done: "+msg.name, false)
		return m, tea.Batch(loadEntries, loadStatus)

	case regenMsg:
		m.flash("regenerated "+msg.name, false)
		// the password changed but the name didn't, so selectCurrent would no-op;
		// force a detail refresh.
		return m, tea.Batch(loadDetail(msg.name), loadStatus)

	case errMsg:
		m.busy = false
		// a locked key fails to decrypt; if we haven't been given a passphrase
		// yet, prompt for one in-app rather than just flashing the error. Don't
		// reopen while already unlocking — startup can fire two locked errors
		// (the auth probe and the auto-selected leaf) and a reopen would wipe a
		// half-typed passphrase.
		if pass.Passphrase == "" && m.mode != modeUnlock && looksLocked(msg.err) {
			return m.openUnlock(), textinput.Blink
		}
		// a non-locked error must not leave us stuck on the pre-auth gate.
		m.ready = true
		m.flash(msg.err.Error(), true)
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

// handleKey dispatches keys by mode.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeCreate:
		return m.handleCreateKey(msg)
	case modeConfirmDelete:
		return m.handleConfirmKey(msg)
	case modeGithub:
		return m.handleGithubKey(msg)
	case modeUnlock:
		return m.handleUnlockKey(msg)
	case modePrompt:
		return m.handlePromptKey(msg)
	case modeCommit:
		return m.handleCommitKey(msg)
	case modeCopy:
		return m.handleCopyKey(msg)
	}
	return m.handleBrowseKey(msg)
}

func (m Model) handleBrowseKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// while the filter input has focus, route keys to it.
	if m.tree.editing {
		cmd := m.tree.updateFilter(msg)
		return m, tea.Batch(cmd, m.selectCurrent())
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "/":
		cmd := m.tree.startFilter()
		return m, tea.Batch(cmd, m.selectCurrent())
	case "esc":
		if m.showHelp {
			m.showHelp = false
			return m, nil
		}
		if m.tree.filtering() {
			m.tree.clearFilter()
			return m, m.selectCurrent()
		}
		return m, nil
	case "j", "down":
		m.tree.down()
		return m, m.selectCurrent()
	case "k", "up":
		m.tree.up()
		return m, m.selectCurrent()
	case "enter", " ":
		// the selected leaf's detail is always shown; enter/space folds a folder
		// or reveals/masks the leaf's password.
		n := m.tree.selected()
		if n == nil {
			return m, nil
		}
		if n.path == "" {
			m.tree.toggle()
			return m, m.selectCurrent()
		}
		m.masked = !m.masked
		m.renderDetail()
		return m, nil
	case "c":
		if m.current == "" {
			return m, nil
		}
		// a password-only entry copies straight away; a multi-field entry opens
		// the picker so the user can choose which value to copy.
		if len(parseFields(m.detailRaw)) <= 1 {
			return m, copyEntry(m.current)
		}
		m.mode = modeCopy
		m.copyCursor = 0
		return m, nil
	case "g":
		// quick path: jump straight into the login form (generated password).
		return m, m.startForm(0)
	case "a":
		// add: choose a type first, then fill it in-app.
		m.mode = modeCreate
		m.genPicking = true
		m.genPreset = 0
		return m, nil
	case "e":
		if m.current != "" {
			return m, m.editExternal(m.current)
		}
	case "p":
		// re-roll the selected entry's password in place, keeping its metadata.
		if m.current != "" {
			return m, regenEntry(m.current, 24, m.symbols)
		}
		return m, nil
	case "r":
		if m.current != "" {
			return m.openPrompt(m.current, false), textinput.Blink
		}
		return m, nil
	case "y":
		if m.current != "" {
			return m.openPrompt(m.current, true), textinput.Blink
		}
		return m, nil
	case "d":
		if m.current != "" {
			m.mode = modeConfirmDelete
		}
		return m, nil
	case "s":
		if !m.hasRepo {
			m.flash("no git repo — use github menu (h) to provision", true)
			return m, nil
		}
		if m.status.Dirty {
			m.flash("commit first — github menu (h) — rebase needs a clean tree", true)
			return m, nil
		}
		m.busy = true
		return m, syncStore
	case "h":
		m.mode = modeGithub
		m.ghView = ghMenu
		m.ghCursor = 0
		return m, nil
	case "u":
		return m.openUnlock(), textinput.Blink
	}

	return m, nil
}

// looksLocked reports whether an error is the gpg "key is locked / can't
// prompt" family, which an in-app passphrase can resolve.
func looksLocked(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "no secret key") ||
		strings.Contains(s, "decryption failed") ||
		strings.Contains(s, "pinentry") ||
		strings.Contains(s, "inappropriate ioctl")
}

// openUnlock switches to the passphrase prompt with a fresh, focused field.
func (m Model) openUnlock() Model {
	m.mode = modeUnlock
	m.passInput.SetValue("")
	m.passInput.Focus()
	return m
}

func (m Model) handleUnlockKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// cancelling still drops to the store; entry names aren't secret, and
		// revealing a password will re-prompt.
		m.mode = modeBrowse
		m.ready = true
		return m, nil
	case "enter":
		pp := m.passInput.Value()
		m.passInput.SetValue("")
		m.mode = modeBrowse
		m.ready = true
		if pp == "" {
			m.flash("passphrase required", true)
			return m, nil
		}
		pass.Passphrase = pp
		return m, m.selectCurrent() // retry decrypt of the highlighted entry
	}
	var cmd tea.Cmd
	m.passInput, cmd = m.passInput.Update(msg)
	return m, cmd
}

// openPrompt switches to the destination-path prompt, prefilled with the source
// name so a rename only needs an edit and a move/copy can retype the folder.
func (m Model) openPrompt(src string, cp bool) Model {
	m.mode = modePrompt
	m.promptSrc = src
	m.promptCopy = cp
	m.promptInput.SetValue(src)
	m.promptInput.CursorEnd()
	m.promptInput.Focus()
	return m
}

func (m Model) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeBrowse
		return m, nil
	case "enter":
		dst := strings.TrimSpace(m.promptInput.Value())
		src, cp := m.promptSrc, m.promptCopy
		m.mode = modeBrowse
		if dst == "" || dst == src {
			m.flash("destination required", true)
			return m, nil
		}
		return m, renameEntry(src, dst, cp)
	}
	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

func (m Model) handleCommitKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeBrowse
		return m, nil
	case "enter":
		msg := m.promptInput.Value()
		m.mode = modeBrowse
		return m, commitStore(msg)
	}
	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

func (m Model) handleCopyKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fields := parseFields(m.detailRaw)
	switch msg.String() {
	case "esc", "q", "c":
		m.mode = modeBrowse
		return m, nil
	case "j", "down":
		if m.copyCursor < len(fields)-1 {
			m.copyCursor++
		}
		return m, nil
	case "k", "up":
		if m.copyCursor > 0 {
			m.copyCursor--
		}
		return m, nil
	case "enter":
		idx := m.copyCursor
		name := m.current
		m.mode = modeBrowse
		// field 0 is the password: route it through `pass -c` for auto-clear.
		// other fields are non-secret values copied directly.
		if idx == 0 {
			return m, copyEntry(name)
		}
		return m, copyField(name, fields[idx])
	}
	return m, nil
}

// preset returns the currently selected preset.
func (m Model) preset() preset { return presets[m.genPreset] }

// focusCount is the number of focusable controls in the form: one per editable
// field, plus the length input when the preset generates a password.
func (m Model) focusCount() int {
	n := len(m.genInputs)
	if m.preset().hasGen() {
		n++
	}
	return n
}

// startForm leaves the picker and builds a blank form for the given preset.
func (m *Model) startForm(idx int) tea.Cmd {
	m.mode = modeCreate
	m.genPicking = false
	m.genPreset = idx
	m.genInputs = nil
	for _, f := range presets[idx].editable() {
		in := textinput.New()
		in.Placeholder = f.label
		m.genInputs = append(m.genInputs, in)
	}
	m.genLen.SetValue("24")
	m.genFocus = 0
	m.focusForm()
	return textinput.Blink
}

// focusForm focuses the control at genFocus and blurs the rest.
func (m *Model) focusForm() {
	for i := range m.genInputs {
		if i == m.genFocus {
			m.genInputs[i].Focus()
		} else {
			m.genInputs[i].Blur()
		}
	}
	if m.preset().hasGen() {
		if m.genFocus == len(m.genInputs) {
			m.genLen.Focus()
		} else {
			m.genLen.Blur()
		}
	}
}

func (m Model) handleCreateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.genPicking {
		switch msg.String() {
		case "esc", "q":
			m.mode = modeBrowse
			return m, nil
		case "j", "down":
			if m.genPreset < len(presets)-1 {
				m.genPreset++
			}
			return m, nil
		case "k", "up":
			if m.genPreset > 0 {
				m.genPreset--
			}
			return m, nil
		case "enter":
			return m, m.startForm(m.genPreset)
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.mode = modeBrowse
		return m, nil
	case "tab", "down":
		m.genFocus = (m.genFocus + 1) % m.focusCount()
		m.focusForm()
		return m, textinput.Blink
	case "shift+tab", "up":
		m.genFocus = (m.genFocus - 1 + m.focusCount()) % m.focusCount()
		m.focusForm()
		return m, textinput.Blink
	case "ctrl+s":
		if m.preset().hasGen() {
			m.symbols = !m.symbols
		}
		return m, nil
	case "enter":
		return m.submitForm()
	}

	// route the key to the focused control.
	var cmd tea.Cmd
	if m.preset().hasGen() && m.genFocus == len(m.genInputs) {
		m.genLen, cmd = m.genLen.Update(msg)
	} else if m.genFocus < len(m.genInputs) {
		m.genInputs[m.genFocus], cmd = m.genInputs[m.genFocus].Update(msg)
	}
	return m, cmd
}

// submitForm collects the form values and fires the create command. genInputs
// is indexed in preset.editable() order, so we walk the preset's fields and
// advance the input cursor for every field except the generated secret.
func (m Model) submitForm() (tea.Model, tea.Cmd) {
	var name, secret string
	var genPw bool
	var metas []kv
	ii := 0
	for _, f := range m.preset().fields {
		switch f.kind {
		case fName:
			name = strings.TrimSpace(m.genInputs[ii].Value())
			ii++
		case fSecret:
			if f.gen {
				genPw = true
			} else {
				secret = m.genInputs[ii].Value()
				ii++
			}
		case fMeta:
			if v := strings.TrimSpace(m.genInputs[ii].Value()); v != "" {
				metas = append(metas, kv{f.key, v})
			}
			ii++
		}
	}
	if name == "" {
		m.flash("name required", true)
		return m, nil
	}
	length, err := strconv.Atoi(strings.TrimSpace(m.genLen.Value()))
	if err != nil || length <= 0 {
		length = 24
	}
	m.mode = modeBrowse
	return m, createEntry(name, genPw, length, m.symbols, secret, metas)
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y":
		name := m.current
		m.mode = modeBrowse
		m.current = ""
		m.detailRaw = ""
		m.renderDetail()
		return m, removeEntry(name)
	default:
		m.mode = modeBrowse
		return m, nil
	}
}

func (m Model) handleGithubKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "h", "q":
		if m.ghView != ghMenu {
			m.ghView = ghMenu
			return m, nil
		}
		m.mode = modeBrowse
		return m, nil
	case "j", "down":
		if m.ghView == ghMenu && m.ghCursor < 4 {
			m.ghCursor++
		}
		return m, nil
	case "k", "up":
		if m.ghView == ghMenu && m.ghCursor > 0 {
			m.ghCursor--
		}
		return m, nil
	case "enter":
		if m.ghView != ghMenu {
			return m, nil
		}
		switch m.ghCursor {
		case 0: // commit the staged changes (prompts for a message)
			if !m.hasRepo {
				m.flash("no git repo yet — provision first", true)
				return m, nil
			}
			if !m.status.Dirty {
				m.flash("nothing to commit", false)
				return m, nil
			}
			m.mode = modeCommit
			m.promptInput.SetValue("")
			m.promptInput.Focus()
			return m, textinput.Blink
		case 1: // sync
			if !m.hasRepo {
				m.flash("no git repo yet — provision first", true)
				return m, nil
			}
			if m.status.Dirty {
				m.flash("commit first — rebase needs a clean tree", true)
				return m, nil
			}
			m.busy = true
			m.mode = modeBrowse
			return m, syncStore
		case 2: // provision a private repo named after the store directory
			m.busy = true
			return m, provision(filepath.Base(pass.StoreDir()))
		case 3: // secret-scanning alerts
			if m.repo.Owner == "" {
				m.flash("no origin remote", true)
				return m, nil
			}
			m.busy = true
			return m, loadAlerts(m.repo)
		case 4: // actions runs
			if m.repo.Owner == "" {
				m.flash("no origin remote", true)
				return m, nil
			}
			m.busy = true
			return m, loadRuns(m.repo)
		}
	}
	return m, nil
}

// editExternal suspends the TUI and runs `pass edit` on an existing entry,
// which opens $EDITOR. New entries are created in-app via the preset form.
func (m Model) editExternal(name string) tea.Cmd {
	cmd := exec.Command("pass", "edit", name)
	cmd.Env = append(os.Environ(), pass.NoAutoCommitEnv)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return editDoneMsg{err}
	})
}

// selectCurrent loads detail for the highlighted entry if it changed. On a
// folder row (or empty tree) it clears the detail pane. It must not mutate the
// model: callers batch it alongside the returned model value.
func (m Model) selectCurrent() tea.Cmd {
	n := m.tree.selected()
	if n == nil || n.path == "" {
		if m.current == "" {
			return nil
		}
		return func() tea.Msg { return clearDetailMsg{} }
	}
	if n.path == m.current {
		return nil
	}
	return loadDetail(n.path)
}

func (m *Model) flash(msg string, isErr bool) {
	m.message = msg
	m.isErr = isErr
	m.messageAt = time.Now()
}

// layout sizes the tree to fill the padded screen below the title and above
// the footer.
func (m *Model) layout() {
	if m.width == 0 {
		return
	}
	treeW := m.width - 2*padX
	if treeW < 1 {
		treeW = 1
	}
	m.treeH = m.height - 2*padY - 4 // title + two blank separators + footer
	if m.treeH < 1 {
		m.treeH = 1
	}
	m.tree.setSize(treeW, m.treeH)
	m.renderDetail()
}

// renderDetail builds the inline detail block for the selected entry, masking
// the first line (the password) unless revealed. The tree always shows it under
// the selected leaf, keyed to detailPath so a reload never shows it elsewhere.
func (m *Model) renderDetail() {
	if m.tree.width == 0 || m.current == "" {
		m.tree.detail = ""
		m.tree.detailPath = ""
		return
	}
	m.tree.detailPath = m.current
	fields := parseFields(m.detailRaw)
	if m.masked {
		fields[0].val = strings.Repeat("•", max(len([]rune(fields[0].val)), 1))
	}
	// the password row carries a reveal hint beside its key (not trailing the
	// masked value, where it reads as part of the dots).
	const revealHint = " (space)"
	key0 := fields[0].key + revealHint
	// align values past the widest key, like the example layout.
	kw := lipgloss.Width(key0)
	for _, f := range fields {
		if w := lipgloss.Width(f.key); w > kw {
			kw = w
		}
	}
	var b strings.Builder
	for i, f := range fields {
		key := f.key
		if i == 0 {
			key = key0
		}
		b.WriteString(theme.Label.Render(key + strings.Repeat(" ", kw-lipgloss.Width(key))))
		b.WriteString("  " + theme.Body.Render(f.val))
		if i < len(fields)-1 {
			b.WriteByte('\n')
		}
	}
	// the tree wraps and indents this block under the leaf when it's open.
	m.tree.detail = b.String()
	m.tree.clampOffset()
}

// field is one display row in the detail dock.
type field struct{ key, val string }

// parseFields splits a decrypted entry into rows: the first line is the
// password, remaining non-empty lines are `key: value` metadata (a line without
// a colon shows under a blank key).
func parseFields(raw string) []field {
	lines := strings.Split(raw, "\n")
	fields := []field{{key: "password"}}
	if len(lines) > 0 {
		fields[0].val = lines[0]
		lines = lines[1:]
	}
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if i := strings.IndexByte(ln, ':'); i >= 0 {
			fields = append(fields, field{strings.TrimSpace(ln[:i]), strings.TrimSpace(ln[i+1:])})
		} else {
			fields = append(fields, field{val: strings.TrimSpace(ln)})
		}
	}
	return fields
}
