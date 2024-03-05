package main

import (
	"discretion/secrets"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
	"github.com/sahilm/fuzzy"
	"golang.design/x/clipboard"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type globalState struct {
	status string
}

var g globalState

type model struct {
	view        map[ViewType]*View
	currentView ViewType

	searchStarted bool
	searchString  string
	cursor        int
	height        int
	start         int
	end           int
}

type Row []string
type Rows []Row

type Column struct {
	Title string
	Width int
}

type Styles struct {
	Header   lipgloss.Style
	Cell     lipgloss.Style
	Selected lipgloss.Style
}

type View struct {
	Header ViewType
	Rows   Rows
	Cols   []Column
}

type ViewType int

const (
	Vaults ViewType = iota
	Secrets
)

func (r Rows) String(i int) string {
	return strings.Join(r[i], " ")
}

func (r Rows) Len() int {
	return len(r)
}

func (v ViewType) String() string {
	switch v {
	case Vaults:
		return "vaults"
	case Secrets:
		return "secrets"
	default:
		return "vaults"
	}
}

func DefaultStyles() Styles {
	return Styles{
		Selected: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")),

		Header: lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			BorderBottom(true).
			Bold(true).
			Padding(0, 1),

		Cell: lipgloss.
			NewStyle().
			Padding(0, 1),
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.searchStarted {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.searchString = ""
				m.searchStarted = false

			case "enter":
				m.FindRows(m.searchString)
				m.searchString = ""
				m.searchStarted = false

			case "backspace":
				m.searchString = m.searchString[:len(m.searchString)-1]

			default:
				cmd = m.ProcessInput(msg.String())
			}
		}
	}

	if !m.searchStarted {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(*m.GetRows())-1 {
					m.cursor++
				}

			case "r":
				m.Refresh()
			case "x":
				m.ToggleView()
			case "/":
				m.searchStarted = true
			case "enter":
				cmd = m.SelectRow()
			}
		}
	}

	m.UpdateRows()

	return m, cmd
}

func (m model) View() string {
	uiString := m.titleView() + "\n"
	uiString += m.headersView() + "\n"

	uiString += m.MapRowsToCols()

	if m.searchStarted {
		uiString += "\n" + m.searchView()
	}

	return baseStyle.Render(uiString) + "\n"
}

func (m *model) titleView() string {
	style := DefaultStyles().Header.
		AlignHorizontal(lipgloss.Center).
		Border(lipgloss.ThickBorder()).
		Italic(true)

	str := make([]string, 0, 2)
	str = append(str, style.Render(fmt.Sprintf("Discretion: %s", m.currentView.String())))

	if g.status != "" {
		str = append(str, style.Foreground(lipgloss.Color("#c3ff68")).Render(g.status))
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, str...)
}

func (m model) headersView() string {
	var s = make([]string, 0, len(*m.GetCols()))
	for _, col := range *m.GetCols() {
		style := lipgloss.NewStyle().
			Width(col.Width).
			MaxWidth(col.Width).
			Inline(true)

		renderedCell := style.Render(runewidth.Truncate(col.Title, col.Width, "…"))
		s = append(s, DefaultStyles().Header.BorderTop(true).Render(renderedCell))
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, s...)
}

func (m model) searchView() string {
	return lipgloss.NewStyle().Render(fmt.Sprintf("Search: %s", m.searchString))
}

func (m *model) ProcessInput(key string) tea.Cmd {
	var cmd tea.Cmd
	m.searchString += key

	return cmd
}

func (m *model) MapRowsToCols() string {
	rows := m.GetRowsRange()
	var r = make([]string, 0, len(rows))

	for i, row := range rows {
		var c = make([]string, 0, len(*m.GetCols()))
		for i, col := range *m.GetCols() {
			style := lipgloss.NewStyle().Width(col.Width).MaxWidth(col.Width).Inline(true)
			renderedCell := style.Render(runewidth.Truncate(row[i], col.Width, "…"))
			c = append(c, DefaultStyles().Cell.Render(renderedCell))
		}

		r = append(r, lipgloss.JoinHorizontal(lipgloss.Left, c...))

		if i == m.cursor-m.start {
			r[i] = DefaultStyles().Selected.Render(r[i])
		}
	}

	return strings.Join(r, "\n")
}

func (m *model) GetSelectedRow() *Row {
	if len(*m.GetRows()) == 0 {
		return nil
	}

	return &(*m.GetRows())[m.cursor]
}

func (m *model) SelectRow() tea.Cmd {
	var cmd tea.Cmd
	row := m.GetSelectedRow()
	if row == nil {
		return cmd
	}

	switch m.currentView {
	case Vaults:
	case Secrets:
		go func() {
			g.status = "Copying..."
			secretValue, success := secrets.TryGetSecret((*row)[2])
			if !success {
				cmd = tea.Println("No secret value found")
			}

			clipboard.Write(clipboard.FmtText, []byte(secretValue.Value))
			g.status = "Copied"
		}()
	}
	return cmd
}

func (m *model) GetRowsRange() []Row {
	if len(*m.GetRows()) == 0 {
		return []Row{}
	}

	return (*m.GetRows())[m.start:m.end]
}

func (m *model) SetCols(cols []Column, view ViewType) {
	m.view[view].Cols = cols
}

func (m *model) GetCols() *[]Column {
	return &m.view[m.currentView].Cols
}

func (m *model) GetRows() *Rows {
	return &m.view[m.currentView].Rows
}

func (m *model) ClearRows() {
	m.view[m.currentView].Rows = []Row{}
}

func (m *model) UpdateRows() {
	if len(*m.GetRows()) <= 10 {
		m.height = len(*m.GetRows())
	} else {
		m.height = 10
	}

	if m.start > len(*m.GetRows()) {
		m.start = len(*m.GetRows())
	}

	m.end = m.start + m.height

	if m.cursor > len(*m.GetRows()) {
		m.cursor = len(*m.GetRows())
	}

	if m.cursor == 0 {
		m.start = 0
	}

	// "Slide" the view window when the cursor is at the top or bottom
	if m.cursor == m.start-1 && m.cursor > 0 {
		m.start--
	}

	if m.cursor == m.end {
		m.start++
	}

	m.end = m.start + m.height

	if m.end > len(*m.GetRows()) {
		m.end = len(*m.GetRows())
	}
}

func (m *model) SetRows(rows []Row, view ViewType) {
	newView := m.view[view]
	if newView.Rows == nil {
		newView.Rows = make([]Row, 0, len(rows))
	}

	newView.Rows = rows

	m.view[view] = newView
}

func (m *model) FindRows(search string) {
	rows := make([]Row, 0, len(*m.GetRows()))
	matches := fuzzy.FindFrom(search, *m.GetRows())

	for _, match := range matches {
		rows = append(rows, (*m.GetRows())[match.Index])
	}

	m.SetRows(rows, m.currentView)
}

func (m *model) Refresh() {
	rows := []Row{}
	for _, vault := range secrets.GetVaults() {
		rows = append(rows, []string{vault.Name, vault.Region, vault.ResourceGroup, vault.Subscription})
	}

	m.SetRows(rows, Vaults)

	rows = []Row{}
	for _, secret := range secrets.GetSecrets() {
		if !secret.Enabled {
			continue
		}

		rows = append(rows, []string{secret.Vault, secret.Name, secret.Identifier})
	}
	m.SetRows(rows, Secrets)
}

func (m *model) ToggleView() {
	if m.currentView == Vaults {
		m.currentView = Secrets
	} else {
		m.currentView = Vaults
	}
}

func (m *model) SwitchView(view ViewType) {
	m.currentView = view
}

func main() {
	secrets.Init()

	m := model{
		view: map[ViewType]*View{
			Vaults:  {},
			Secrets: {},
		},

		currentView: Vaults,
	}

	m.SetCols([]Column{
		{Title: "Name", Width: 20},
		{Title: "Region", Width: 20},
		{Title: "Resource Group", Width: 20},
	}, Vaults)

	m.SetCols([]Column{
		{Title: "Vault", Width: 20},
		{Title: "Name", Width: 20},
	}, Secrets)

	m.start = 0
	m.cursor = 0
	m.end = 0

	m.Refresh()

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
