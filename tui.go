package main

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	treeStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2)

	extractStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2)

	searchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Padding(0, 1)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFF00")).
				Bold(true)
)

// TreeItem represents an item in the JSON tree
type TreeItem struct {
	key     string
	display string
	depth   int
}

// Model represents the application state
type Model struct {
	// JSON data
	jsonData []byte
	fileName string

	// JSON Processor
	jp *JSONProcessor

	// Tree state
	treeItems   []TreeItem
	selectedIdx int

	// Search state
	searchQuery  string
	filteredKeys []string

	// UI state
	width       int
	height      int
	leftWidth   int
	rightWidth  int
	ready       bool

	// Viewports
	treeViewport    viewport.Model
	extractViewport viewport.Model
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "up", "ctrl+p":
			if m.selectedIdx > 0 {
				m.selectedIdx--
				if m.selectedIdx >= 0 && m.selectedIdx < len(m.filteredKeys) {
					m.searchQuery = m.filteredKeys[m.selectedIdx]
				}
				m.updateTreeContent()
				m.updateExtractContent()
			}

		case "down", "ctrl+n":
			if m.selectedIdx < len(m.filteredKeys)-1 {
				m.selectedIdx++
				if m.selectedIdx >= 0 && m.selectedIdx < len(m.filteredKeys) {
					m.searchQuery = m.filteredKeys[m.selectedIdx]
				}
				m.updateTreeContent()
				m.updateExtractContent()
			}

		case "enter":
			if len(m.filteredKeys) > 0 && m.selectedIdx < len(m.filteredKeys) {
				m.searchQuery = m.filteredKeys[m.selectedIdx]
			}

		case "backspace", "ctrl+h":
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.updateFilteredKeys()
			}

		default:
			// Handle character input for search
			if len(msg.String()) == 1 {
				char := msg.String()[0]
				if (char >= 'a' && char <= 'z') ||
					(char >= 'A' && char <= 'Z') ||
					(char >= '0' && char <= '9') ||
					char == '.' || char == '[' || char == ']' ||
					char == '_' || char == '#' {
					m.searchQuery += msg.String()
					m.updateFilteredKeys()
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate dynamic tree width based on content
		m.calculateTreeWidth()

		if !m.ready {
			m.treeViewport = viewport.New(viewport.WithWidth(m.leftWidth-4), viewport.WithHeight(m.height-8))
			m.extractViewport = viewport.New(viewport.WithWidth(m.rightWidth-4), viewport.WithHeight(m.height-8))
			m.ready = true
		} else {
			m.treeViewport.SetWidth(m.leftWidth - 4)
			m.treeViewport.SetHeight(m.height - 8)
			m.extractViewport.SetWidth(m.rightWidth - 4)
			m.extractViewport.SetHeight(m.height - 8)
		}

		m.updateTreeContent()
		m.updateExtractContent()
	}

	return m, nil
}

// View renders the UI
func (m Model) View() tea.View {
	if !m.ready {
		return tea.NewView("Initializing...")
	}

	header := m.renderHeader()
	main := m.renderMain()
	footer := m.renderFooter()

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		main,
		footer,
	)
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// renderHeader renders the header with filename
func (m Model) renderHeader() string {
	return headerStyle.Render(fmt.Sprintf("File: %s", m.fileName))
}

// renderMain renders the main content (left and right panels)
func (m Model) renderMain() string {
	leftPanel := m.renderTreePanel()
	rightPanel := m.renderExtractPanel()

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		rightPanel,
	)
}

// renderTreePanel renders the left panel with JSON tree
func (m Model) renderTreePanel() string {
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Render("JSON Tree")

	content := m.treeViewport.View()

	panel := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		content,
	)

	return treeStyle.
		Width(m.leftWidth - 4).
		Height(m.height - 8).
		Render(panel)
}

// renderExtractPanel renders the right panel with JSON extraction
func (m Model) renderExtractPanel() string {
	title := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Render("JSON Extractor")

	content := m.extractViewport.View()

	panel := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		content,
	)

	return extractStyle.
		Width(m.rightWidth - 4).
		Height(m.height - 8).
		Render(panel)
}

// renderFooter renders the search bar
func (m Model) renderFooter() string {
	searchText := fmt.Sprintf("🔍 Search: %s", m.searchQuery)
	return searchStyle.Render(searchText)
}

// updateFilteredKeys updates the filtered keys based on search query
func (m *Model) updateFilteredKeys() {
	if m.searchQuery == "" {
		m.filteredKeys = make([]string, len(m.jp.keys))
		copy(m.filteredKeys, m.jp.keys)
	} else {
		m.filteredKeys = []string{}
		for _, key := range m.jp.keys {
			if fuzzyFind(key, m.searchQuery) {
				m.filteredKeys = append(m.filteredKeys, key)
			}
		}
	}

	// Sort filtered keys to group children of the same parent together
	sortTreeKeys(m.filteredKeys)

	if len(m.filteredKeys) == 0 {
		m.selectedIdx = -1
	} else if m.selectedIdx >= len(m.filteredKeys) {
		m.selectedIdx = 0
	}

	// Recalculate tree width based on filtered content
	m.calculateTreeWidth()
	m.updateTreeContent()
	m.updateExtractContent()
}

// updateTreeContent updates the tree viewport content
func (m *Model) updateTreeContent() {
	var content strings.Builder

	for i, key := range m.filteredKeys {
		display := m.formatTreeItem(key, i == m.selectedIdx)
		content.WriteString(display)
		content.WriteString("\n")
	}

	m.treeViewport.SetContent(content.String())

	// Auto-scroll to selected item
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.filteredKeys) {
		lineHeight := 1
		targetLine := m.selectedIdx * lineHeight
		viewportHeight := m.treeViewport.Height()

		// Scroll down if selected item is below viewport
		if targetLine >= m.treeViewport.YOffset()+viewportHeight {
			m.treeViewport.SetYOffset(targetLine - viewportHeight + 1)
		}
		// Scroll up if selected item is above viewport
		if targetLine < m.treeViewport.YOffset() {
			m.treeViewport.SetYOffset(targetLine)
		}
	}
}

// formatTreeItem formats a tree item with proper indentation and highlighting
func (m *Model) formatTreeItem(key string, selected bool) string {
	depth := strings.Count(key, ".") + strings.Count(key, "[")
	indent := strings.Repeat("  ", depth)

	// Determine the display symbol
	symbol := "├─"

	// Extract display name with parent context for clarity
	displayName := getDisplayName(key)

	display := fmt.Sprintf("%s%s %s", indent, symbol, displayName)

	if selected {
		return selectedItemStyle.Render("> " + display)
	}
	return "  " + display
}

// getDisplayName extracts a meaningful display name from the full key path
func getDisplayName(key string) string {
	parts := strings.Split(key, ".")
	lastPart := parts[len(parts)-1]
	return lastPart
}

// updateExtractContent updates the extract viewport content
func (m *Model) updateExtractContent() {
	if m.selectedIdx >= 0 && m.selectedIdx < len(m.filteredKeys) {
		selectedKey := m.filteredKeys[m.selectedIdx]
		jsonData := getParsedResult(selectedKey, m.jsonData)
		highlightedJSON := highlightJSON(jsonData)
		m.extractViewport.SetContent(highlightedJSON)
	} else {
		m.extractViewport.SetContent("No item selected")
	}
}

// calculateTreeWidth calculates the optimal tree width based on content
func (m *Model) calculateTreeWidth() {
	if m.width == 0 {
		return
	}

	// Find maximum display width needed for tree items
	maxWidth := 0
	for i, key := range m.filteredKeys {
		display := m.formatTreeItemPlain(key, i == m.selectedIdx)
		displayWidth := lipgloss.Width(display)
		if displayWidth > maxWidth {
			maxWidth = displayWidth
		}
	}

	// Add padding for borders and margins
	neededWidth := maxWidth + 8

	// Calculate percentage based on needed width
	minWidth := int(float64(m.width) * 0.25) // 25% minimum
	maxWidthLimit := int(float64(m.width) * 0.60) // 60% maximum

	// Set tree width within bounds
	if neededWidth < minWidth {
		m.leftWidth = minWidth
	} else if neededWidth > maxWidthLimit {
		m.leftWidth = maxWidthLimit
	} else {
		m.leftWidth = neededWidth
	}

	m.rightWidth = m.width - m.leftWidth - 4

	// Update viewport widths if they exist
	if m.ready {
		m.treeViewport.SetWidth(m.leftWidth - 4)
		m.extractViewport.SetWidth(m.rightWidth - 4)
	}
}

// formatTreeItemPlain formats a tree item without styling for width calculation
func (m *Model) formatTreeItemPlain(key string, selected bool) string {
	depth := strings.Count(key, ".") + strings.Count(key, "[")
	indent := strings.Repeat("  ", depth)

	symbol := "├─"

	displayName := getDisplayName(key)

	display := fmt.Sprintf("%s%s %s", indent, symbol, displayName)

	if selected {
		return "> " + display
	}
	return "  " + display
}

// sortTreeKeys sorts keys to group children of the same parent together
func sortTreeKeys(keys []string) {
	// Simple alphabetical sort will group keys with the same prefix together
	for i := 0; i < len(keys)-1; i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] > keys[j] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
}

// generateTreeItems generates tree items from JSON keys
func generateTreeItems(keys []string) []TreeItem {
	items := make([]TreeItem, 0, len(keys))

	for _, key := range keys {
		depth := strings.Count(key, ".") + strings.Count(key, "[")
		display := formatKeyAsTree(key)

		items = append(items, TreeItem{
			key:     key,
			display: display,
			depth:   depth,
		})
	}

	return items
}

// formatKeyAsTree formats a key as a tree-like display
func formatKeyAsTree(key string) string {
	depth := strings.Count(key, ".") + strings.Count(key, "[")
	indent := strings.Repeat("  ", depth)

	parts := strings.Split(key, ".")
	lastPart := parts[len(parts)-1]

	return fmt.Sprintf("%s├─ %s", indent, lastPart)
}

// NewBubbleteaModel creates a new Bubbletea model
func NewBubbleteaModel(jp *JSONProcessor, fileName string) Model {
	// Sort keys initially
	sortTreeKeys(jp.keys)

	m := Model{
		jsonData:     jp.jsonData,
		fileName:     fileName,
		jp:           jp,
		filteredKeys: jp.keys,
		selectedIdx:  0,
		searchQuery:  "",
	}

	m.treeItems = generateTreeItems(jp.keys)

	return m
}

// RunBubbleteaTUI starts the Bubbletea TUI
func RunBubbleteaTUI(jp *JSONProcessor, fileName string) error {
	m := NewBubbleteaModel(jp, fileName)

	p := tea.NewProgram(m)
	_, err := p.Run()
	return err
}
