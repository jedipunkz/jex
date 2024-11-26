package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/quick"
	"github.com/jroimartin/gocui"
	"github.com/tidwall/gjson"
)

type TUIManager struct {
	gui           *gocui.Gui
	searchQuery   string
	selectedIndex int
	filteredKeys  []string
	jp            *JSONProcessor
}

// run TUI
func (tui *TUIManager) run() {
	var err error
	tui.gui, err = gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		panic(err)
	}
	defer tui.gui.Close()

	tui.filteredKeys = tui.jp.keys
	tui.selectedIndex = 0

	tui.gui.SetManagerFunc(tui.layout)

	tui.setKeybindings()

	if err := tui.gui.MainLoop(); err != nil && err != gocui.ErrQuit {
		fmt.Println("Error:", err)
	}
}

func (tui *TUIManager) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	// query view
	vQuery, err := g.SetView("query", 0, 0, maxX-1, 3)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	vQuery.Clear()
	fmt.Fprintf(vQuery, "Search Query: %s", tui.searchQuery)

	// candidates view
	vCandidates, err := g.SetView("candidates", 0, 4, maxX-1, 15)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	vCandidates.Clear()
	for i, key := range tui.filteredKeys {
		if i == tui.selectedIndex {
			fmt.Fprintf(vCandidates, "\033[33m> %s\033[0m\n", key) // Yellow color
		} else {
			fmt.Fprintf(vCandidates, "  %s\n", key)
		}
	}

	// adjust scroll position
	if tui.selectedIndex >= 0 && tui.selectedIndex < len(tui.filteredKeys) {
		_, oy := vCandidates.Origin()
		_, sy := vCandidates.Size()
		if tui.selectedIndex >= oy+sy {
			if err := vCandidates.SetOrigin(0, tui.selectedIndex-sy+1); err != nil {
				return err
			}
		} else if tui.selectedIndex < oy {
			if err := vCandidates.SetOrigin(0, tui.selectedIndex); err != nil {
				return err
			}
		}
	}

	// json view
	vJSON, err := g.SetView("json", 0, 16, maxX-1, maxY-1)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	vJSON.Clear()
	if tui.selectedIndex >= 0 && tui.selectedIndex < len(tui.filteredKeys) {
		jsonData := getParsedResult(tui.filteredKeys[tui.selectedIndex], tui.jp.jsonData)
		highlightedJSON := highlightJSON(jsonData)
		fmt.Fprintln(vJSON, highlightedJSON)
	}

	return nil
}

func highlightJSON(jsonData string) string {
	var highlighted bytes.Buffer
	err := quick.Highlight(&highlighted, jsonData, "json", "terminal", "monokai")
	if err != nil {
		return jsonData
	}
	return highlighted.String()
}

func (tui *TUIManager) setKeybindings() {
	// Quit
	if err := tui.gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	}); err != nil {
		panic(err)
	}
	if err := tui.gui.SetKeybinding("", gocui.KeyCtrlN, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(tui.filteredKeys) > 0 {
			tui.selectedIndex = (tui.selectedIndex + 1) % len(tui.filteredKeys)
		}
		g.Update(func(g *gocui.Gui) error { return nil })
		return nil
	}); err != nil {
		panic(err)
	}

	if err := tui.gui.SetKeybinding("", gocui.KeyCtrlP, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(tui.filteredKeys) > 0 {
			tui.selectedIndex = (tui.selectedIndex - 1 + len(tui.filteredKeys)) % len(tui.filteredKeys)
		}
		g.Update(func(g *gocui.Gui) error { return nil })
		return nil
	}); err != nil {
		panic(err)
	}

	// ctrl+h backspace
	if err := tui.gui.SetKeybinding("", gocui.KeyCtrlH, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(tui.searchQuery) > 0 {
			tui.searchQuery = tui.searchQuery[:len(tui.searchQuery)-1]
		}
		tui.filteredKeys = updateSelectedIndex(&tui.searchQuery, tui.jp.keys, &tui.selectedIndex)
		g.Update(func(g *gocui.Gui) error { return nil })
		return nil
	}); err != nil {
		panic(err)
	}

	// enter key to select the candidate
	if err := tui.gui.SetKeybinding("", gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(tui.filteredKeys) > 0 {
			tui.searchQuery = tui.filteredKeys[tui.selectedIndex]
		}
		g.Update(func(g *gocui.Gui) error { return nil })
		return nil
	}); err != nil {
		panic(err)
	}

	// input string to edit the query
	for char := rune('a'); char <= rune('z'); char++ {
		char := char
		if err := tui.gui.SetKeybinding("", char, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			tui.searchQuery += string(char)
			tui.filteredKeys = updateSelectedIndex(&tui.searchQuery, tui.jp.keys, &tui.selectedIndex)
			g.Update(func(g *gocui.Gui) error { return nil })
			return nil
		}); err != nil {
			panic(err)
		}
	}
	for char := rune('0'); char <= rune('9'); char++ {
		char := char
		if err := tui.gui.SetKeybinding("", char, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			tui.searchQuery += string(char)
			tui.filteredKeys = updateSelectedIndex(&tui.searchQuery, tui.jp.keys, &tui.selectedIndex)
			g.Update(func(g *gocui.Gui) error { return nil })
			return nil
		}); err != nil {
			panic(err)
		}
	}

	// control characters support
	specialChars := []rune{'[', ']', '.', '_', '#'}
	for _, char := range specialChars {
		char := char
		if err := tui.gui.SetKeybinding("", char, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			tui.searchQuery += string(char)
			tui.filteredKeys = updateSelectedIndex(&tui.searchQuery, tui.jp.keys, &tui.selectedIndex)
			g.Update(func(g *gocui.Gui) error { return nil })
			return nil
		}); err != nil {
			panic(err)
		}
	}
}

// update candidates based on the query string
func updateSelectedIndex(searchQuery *string, keys []string, selectedIndex *int) []string {
	var filteredKeys []string
	for _, key := range keys {
		if fuzzyFind(key, *searchQuery) {
			filteredKeys = append(filteredKeys, key)
		}
	}

	if len(filteredKeys) == 0 {
		*selectedIndex = -1
	} else {
		*selectedIndex = 0
	}

	return filteredKeys
}

// fuzzy match function to check if all characters in searchQuery are in key in order
func fuzzyFind(key, searchQuery string) bool {
	keyIndex := 0
	for _, char := range searchQuery {
		found := false
		for keyIndex < len(key) {
			if key[keyIndex] == byte(char) {
				found = true
				keyIndex++
				break
			}
			keyIndex++
		}
		if !found {
			return false
		}
	}
	return true
}

// Removed displayParsedResult function as it was just returning getParsedResult

func getParsedResult(query string, jsonData []byte) string {
	if strings.Contains(query, "[]") {
		return handleArrayQuery(query, jsonData)
	}

	if strings.Contains(query, "[") && strings.Contains(query, "]") {
		return handleIndexedQuery(query, jsonData)
	}

	return handleOrdinaryQuery(query, jsonData)
}

func handleArrayQuery(query string, jsonData []byte) string {
	baseQuery := strings.Split(query, "[]")[0]
	field := strings.TrimPrefix(strings.Split(query, "[]")[1], ".")
	arrayResult := gjson.GetBytes(jsonData, baseQuery)
	if arrayResult.IsArray() {
		var values []string
		arrayResult.ForEach(func(_, val gjson.Result) bool {
			if strings.Contains(field, "[") && strings.Contains(field, "]") {
				values = append(values, handleNestedArray(val, field)...)
			} else {
				values = append(values, val.Get(field).String())
			}
			return true
		})
		return strings.Join(values, "\n")
	}
	return "Query failed. No matching data found."
}

func handleNestedArray(val gjson.Result, field string) []string {
	var values []string
	nestedBase := strings.Split(field, "[")[0]
	nestedIndex := strings.Split(strings.Split(field, "[")[1], "]")[0]
	nestedField := ""
	if strings.Contains(field, "]") {
		nestedField = strings.TrimPrefix(strings.Split(field, "]")[1], ".")
	}
	nestedArray := val.Get(nestedBase)
	if nestedArray.IsArray() {
		nestedArray.ForEach(func(index, nestedVal gjson.Result) bool {
			if index.String() == nestedIndex {
				if nestedField != "" && strings.Contains(nestedField, "[") && strings.Contains(nestedField, "]") {
					values = append(values, handleNestedArray(nestedVal, nestedField)...)
				} else if nestedField != "" {
					values = append(values, nestedVal.Get(nestedField).String())
				} else {
					values = append(values, nestedVal.String())
				}
			}
			return true
		})
	}
	return values
}

func handleIndexedQuery(query string, jsonData []byte) string {
	baseQuery, field := splitQuery(query)
	baseQuery = strings.Replace(baseQuery, "[", ".", -1)
	baseQuery = strings.Replace(baseQuery, "]", "", -1)
	arrayResult := gjson.GetBytes(jsonData, baseQuery)
	if arrayResult.Exists() {
		if field != "" {
			if strings.Contains(field, "[") && strings.Contains(field, "]") {
				return strings.Join(handleNestedIndexedQuery(arrayResult, field), "\n")
			}
			return arrayResult.Get(field).String()
		}
		return arrayResult.String()
	}
	return "Query failed. No matching data found."
}

func handleNestedIndexedQuery(arrayResult gjson.Result, field string) []string {
	fieldBase, fieldField := splitQuery(field)
	fieldBase = strings.Replace(fieldBase, "[", ".", -1)
	fieldBase = strings.Replace(fieldBase, "]", "", -1)
	nestedResult := arrayResult.Get(fieldBase)
	if nestedResult.Exists() {
		if fieldField != "" {
			if strings.Contains(fieldField, "[") && strings.Contains(fieldField, "]") {
				return handleNestedArray(nestedResult, fieldField)
			}
			return []string{nestedResult.Get(fieldField).String()}
		}
		return []string{nestedResult.String()}
	}
	return []string{"Query failed. No matching data found."}
}

func handleOrdinaryQuery(query string, jsonData []byte) string {
	result := gjson.GetBytes(jsonData, query)
	if result.Exists() {
		if result.IsObject() || result.IsArray() {
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, []byte(result.Raw), "", "  "); err == nil {
				return prettyJSON.String()
			}
			return result.Raw
		}
		return result.String()
	}
	return "Query failed. No matching data found."
}

func splitQuery(query string) (string, string) {
	if dotIndex := strings.Index(query, "."); dotIndex != -1 {
		return query[:dotIndex], query[dotIndex+1:]
	}
	return query, ""
}
