package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles define the visual appearance of UI components
var (
	// headerStyle styles the top header bar showing the filename
	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	// treeStyle styles the left panel containing the JSON tree structure
	treeStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2)

	// extractStyle styles the right panel showing extracted JSON values
	extractStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1, 2)

	// searchStyle styles the search input bar at the bottom
	searchStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Padding(0, 1)

	// selectedItemStyle highlights the currently selected tree item
	selectedItemStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFF00")).
				Bold(true)
)

// TreeItem represents an item in the JSON tree structure
// Each item contains its key path, display format, and nesting depth
type TreeItem struct {
	key     string // Full JSON path (e.g., "user.address.city")
	display string // Formatted display text with tree symbols
	depth   int    // Nesting level in the JSON hierarchy
}

// Model represents the complete application state for the TUI
// It manages JSON data, tree navigation, search functionality, and UI layout
type Model struct {
	// JSON data and processing
	jsonData []byte         // Raw JSON input data
	fileName string         // Source filename or "stdin"
	jp       *JSONProcessor // JSON processor for key extraction and queries

	// Tree navigation state
	treeItems   []TreeItem // All tree items generated from JSON keys
	selectedIdx int        // Currently selected item index
	filteredKeys []string  // Keys matching current search query

	// Search functionality
	searchQuery string // Current search/filter text

	// UI dimensions and state
	width      int  // Terminal width
	height     int  // Terminal height
	leftWidth  int  // Width of left panel (tree)
	rightWidth int  // Width of right panel (extractor)
	ready      bool // Whether UI is initialized

	// Viewports for scrollable content
	treeViewport    viewport.Model // Scrollable tree view
	extractViewport viewport.Model // Scrollable extraction view
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Calculate dynamic tree width based on content
		m.calculateTreeWidth()

		if !m.ready {
			m.treeViewport = viewport.New(m.leftWidth-4, m.height-8)
			m.extractViewport = viewport.New(m.rightWidth-4, m.height-8)
			m.ready = true
		} else {
			m.treeViewport.Width = m.leftWidth - 4
			m.treeViewport.Height = m.height - 8
			m.extractViewport.Width = m.rightWidth - 4
			m.extractViewport.Height = m.height - 8
		}

		m.updateTreeContent()
		m.updateExtractContent()
	}

	return m, nil
}

// handleKeyPress processes keyboard input and updates the model accordingly
// This separates key handling logic from the main Update function for better maintainability
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "up", "ctrl+p":
		return m.handleNavigationUp(), nil

	case "down", "ctrl+n":
		return m.handleNavigationDown(), nil

	case "enter":
		return m.handleEnterKey(), nil

	case "backspace", "ctrl+h":
		return m.handleBackspace(), nil

	default:
		return m.handleCharacterInput(msg), nil
	}
}

// handleNavigationUp moves the selection up in the tree
func (m Model) handleNavigationUp() Model {
	if m.selectedIdx > 0 {
		m.selectedIdx--
		if m.selectedIdx >= 0 && m.selectedIdx < len(m.filteredKeys) {
			m.searchQuery = m.filteredKeys[m.selectedIdx]
		}
		m.updateTreeContent()
		m.updateExtractContent()
	}
	return m
}

// handleNavigationDown moves the selection down in the tree
func (m Model) handleNavigationDown() Model {
	if m.selectedIdx < len(m.filteredKeys)-1 {
		m.selectedIdx++
		if m.selectedIdx >= 0 && m.selectedIdx < len(m.filteredKeys) {
			m.searchQuery = m.filteredKeys[m.selectedIdx]
		}
		m.updateTreeContent()
		m.updateExtractContent()
	}
	return m
}

// handleEnterKey confirms the current selection
func (m Model) handleEnterKey() Model {
	if len(m.filteredKeys) > 0 && m.selectedIdx < len(m.filteredKeys) {
		m.searchQuery = m.filteredKeys[m.selectedIdx]
	}
	return m
}

// handleBackspace removes the last character from the search query
func (m Model) handleBackspace() Model {
	if len(m.searchQuery) > 0 {
		m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
		m.updateFilteredKeys()
	}
	return m
}

// handleCharacterInput processes character input for search
func (m Model) handleCharacterInput(msg tea.KeyMsg) Model {
	if len(msg.String()) == 1 {
		char := msg.String()[0]
		if isValidSearchChar(char) {
			m.searchQuery += msg.String()
			m.updateFilteredKeys()
		}
	}
	return m
}

// isValidSearchChar checks if a character is valid for search input
func isValidSearchChar(char byte) bool {
	return (char >= 'a' && char <= 'z') ||
		(char >= 'A' && char <= 'Z') ||
		(char >= '0' && char <= '9') ||
		char == '.' || char == '[' || char == ']' ||
		char == '_' || char == '#'
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	header := m.renderHeader()
	main := m.renderMain()
	footer := m.renderFooter()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		main,
		footer,
	)
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

// createPanel creates a styled panel with title and content
// This common function reduces code duplication between tree and extract panels
func createPanel(title string, content string, style lipgloss.Style, width, height int) string {
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Render(title)

	panel := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle,
		content,
	)

	return style.
		Width(width - 4).
		Height(height - 8).
		Render(panel)
}

// renderTreePanel renders the left panel with JSON tree
func (m Model) renderTreePanel() string {
	return createPanel("JSON Tree", m.treeViewport.View(), treeStyle, m.leftWidth, m.height)
}

// renderExtractPanel renders the right panel with JSON extraction
func (m Model) renderExtractPanel() string {
	return createPanel("JSON Extractor", m.extractViewport.View(), extractStyle, m.rightWidth, m.height)
}

// renderFooter renders the search bar
func (m Model) renderFooter() string {
	searchText := fmt.Sprintf("🔍 Search: %s", m.searchQuery)
	return searchStyle.Render(searchText)
}

// updateFilteredKeys filters the JSON keys based on the current search query
// Uses fuzzy matching to find keys containing the search characters in order
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

// updateTreeContent refreshes the tree viewport with current filtered keys
// Automatically scrolls to keep the selected item visible
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
		viewportHeight := m.treeViewport.Height

		// Scroll down if selected item is below viewport
		if targetLine >= m.treeViewport.YOffset+viewportHeight {
			m.treeViewport.SetYOffset(targetLine - viewportHeight + 1)
		}
		// Scroll up if selected item is above viewport
		if targetLine < m.treeViewport.YOffset {
			m.treeViewport.SetYOffset(targetLine)
		}
	}
}

// formatTreeItem formats a tree item with proper indentation and highlighting
func (m *Model) formatTreeItem(key string, selected bool) string {
	display := m.buildTreeItemDisplay(key)
	if selected {
		return selectedItemStyle.Render("> " + display)
	}
	return "  " + display
}

// buildTreeItemDisplay builds the display string for a tree item
// This shared function eliminates duplication between formatTreeItem and formatTreeItemPlain
func (m *Model) buildTreeItemDisplay(key string) string {
	depth := strings.Count(key, ".") + strings.Count(key, "[")
	indent := strings.Repeat("  ", depth)
	symbol := "├─"
	displayName := getDisplayName(key)
	return fmt.Sprintf("%s%s %s", indent, symbol, displayName)
}

// getDisplayName extracts a meaningful display name from the full key path
func getDisplayName(key string) string {
	parts := strings.Split(key, ".")
	lastPart := parts[len(parts)-1]
	return lastPart
}

// updateExtractContent refreshes the extract viewport with the selected key's JSON value
// Shows syntax-highlighted JSON for the currently selected item
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

// calculateTreeWidth dynamically adjusts the tree panel width based on content
// Ensures the tree is wide enough to display items but doesn't take too much space
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
		m.treeViewport.Width = m.leftWidth - 4
		m.extractViewport.Width = m.rightWidth - 4
	}
}

// formatTreeItemPlain formats a tree item without styling for width calculation
func (m *Model) formatTreeItemPlain(key string, selected bool) string {
	display := m.buildTreeItemDisplay(key)
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

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
