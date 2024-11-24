package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/tidwall/gjson"
)

type JSONProcessor struct {
	jsonData []byte
	keys     []string
}

type TUIManager struct {
	gui           *gocui.Gui
	searchQuery   string
	selectedIndex int
	filteredKeys  []string
	jp            *JSONProcessor
}

func main() {
	var jsonStr strings.Builder

	if len(os.Args) > 1 {
		filePath := os.Args[1]
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Println("Error reading file:", err)
			return
		}
		jsonStr.Write(data)
	} else {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				jsonStr.WriteString(scanner.Text() + "\n")
			}
		} else {
			fmt.Println("Usage: jex <JSON_FILE> or cat <JSON_FILE> | jex")
			return
		}
	}

	jp := &JSONProcessor{
		jsonData: []byte(jsonStr.String()),
	}

	jp.extractKeys()

	tui := &TUIManager{
		jp: jp,
	}

	tui.run()
}

// JSONProcessor extract keys from JSON data
func (jp *JSONProcessor) extractKeys() {
	var walk func(prefix string, value gjson.Result)
	walk = func(prefix string, value gjson.Result) {
		if value.IsObject() {
			value.ForEach(func(key, val gjson.Result) bool {
				fullKey := key.String()
				if prefix != "" {
					fullKey = prefix + "." + fullKey
				}
				jp.keys = append(jp.keys, fullKey)
				walk(fullKey, val)
				return true
			})
		} else if value.IsArray() {
			value.ForEach(func(_, val gjson.Result) bool {
				walk(prefix+"[]", val)
				return true
			})
		}
	}

	walk("", gjson.ParseBytes(jp.jsonData))

	// 不要な候補を削除
	jp.keys = filterInvalidKeys(jp.keys)
}

// remove invalid keys (e.g. "foo[]")
func filterInvalidKeys(keys []string) []string {
	var validKeys []string
	for _, key := range keys {
		if !strings.HasSuffix(key, "[]") { // "[]" で終わるキーは除外
			validKeys = append(validKeys, key)
		}
	}
	return validKeys
}

// run TUI
func (tui *TUIManager) run() {
	var err error
	tui.gui, err = gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		panic(err)
	}
	defer tui.gui.Close()

	tui.gui.SetManagerFunc(tui.layout)

	// キーバインド設定
	tui.setKeybindings()

	if err := tui.gui.MainLoop(); err != nil && err != gocui.ErrQuit {
		fmt.Println("Error:", err)
	}
}

func (tui *TUIManager) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	vQuery, err := g.SetView("query", 0, 0, maxX-1, 3)
	if err != nil && err != gocui.ErrUnknownView {
		return err
	}
	vQuery.Clear()
	fmt.Fprintf(vQuery, "Search Query: %s", tui.searchQuery)

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
		// 文字列やその他の型
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
		displayParsedResult(vJSON, tui.filteredKeys[tui.selectedIndex], tui.jp.jsonData)
	}

	return nil
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
	specialChars := []rune{'[', ']', '.', '_'}
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
		if strings.Contains(key, *searchQuery) {
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

func displayParsedResult(v *gocui.View, query string, jsonData []byte) {
	result := getParsedResult(query, jsonData)
	fmt.Fprintln(v, result)
}

func getParsedResult(query string, jsonData []byte) string {
	if strings.Contains(query, "[]") {
		// e.g. "foo[].bar" -> display all "bar" fields in "foo" array
		baseQuery := strings.Split(query, "[]")[0]
		field := strings.TrimPrefix(strings.Split(query, "[]")[1], ".")
		arrayResult := gjson.GetBytes(jsonData, baseQuery)
		if arrayResult.IsArray() {
			var values []string
			arrayResult.ForEach(func(_, val gjson.Result) bool {
				values = append(values, val.Get(field).String())
				return true
			})
			return strings.Join(values, "\n")
		}
	}

	// ordinary query processing
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
