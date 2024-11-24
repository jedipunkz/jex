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

func main() {
	// JSON データの読み込み
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

	runTUI(jp)
}

// JSON のキー候補を抽出する
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

// 不正なキーを除外する（例: "contributors[]"）
func filterInvalidKeys(keys []string) []string {
	var validKeys []string
	for _, key := range keys {
		if !strings.HasSuffix(key, "[]") { // "[]" で終わるキーは除外
			validKeys = append(validKeys, key)
		}
	}
	return validKeys
}

func runTUI(jp *JSONProcessor) {
	var searchQuery string
	var selectedIndex int
	var filteredKeys []string

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		panic(err)
	}
	defer g.Close()

	g.SetManagerFunc(func(g *gocui.Gui) error {
		maxX, maxY := g.Size()

		// クエリ入力ビュー
		vQuery, err := g.SetView("query", 0, 0, maxX-1, 3)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		vQuery.Clear()
		fmt.Fprintf(vQuery, "Search Query: %s", searchQuery)

		// 候補表示ビュー
		vCandidates, err := g.SetView("candidates", 0, 4, maxX-1, 15)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		vCandidates.Clear()
		for i, key := range filteredKeys {
			if i == selectedIndex {
				fmt.Fprintf(vCandidates, "> %s\n", key)
			} else {
				fmt.Fprintf(vCandidates, "  %s\n", key)
			}
		}

		// スクロール位置を調整
		if selectedIndex >= 0 && selectedIndex < len(filteredKeys) {
			_, oy := vCandidates.Origin()
			_, sy := vCandidates.Size()
			if selectedIndex >= oy+sy {
				vCandidates.SetOrigin(0, selectedIndex-sy+1)
			} else if selectedIndex < oy {
				vCandidates.SetOrigin(0, selectedIndex)
			}
		}

		// JSON 表示ビュー
		vJSON, err := g.SetView("json", 0, 16, maxX-1, maxY-1)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		vJSON.Clear()
		if selectedIndex >= 0 && selectedIndex < len(filteredKeys) {
			displayParsedResult(vJSON, filteredKeys[selectedIndex], jp.jsonData)
		}

		return nil
	})

	// キーバインド設定
	setKeybindings(g, jp, &searchQuery, &selectedIndex, &filteredKeys)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		fmt.Println("Error:", err)
	}
}

func setKeybindings(g *gocui.Gui, jp *JSONProcessor, searchQuery *string, selectedIndex *int, filteredKeys *[]string) {
	// Quit
	g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	})

	// 候補選択を Ctrl+N / Ctrl+P で操作
	g.SetKeybinding("", gocui.KeyCtrlN, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(*filteredKeys) > 0 {
			*selectedIndex = (*selectedIndex + 1) % len(*filteredKeys)
		}
		g.Update(func(g *gocui.Gui) error { return nil })
		return nil
	})
	g.SetKeybinding("", gocui.KeyCtrlP, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(*filteredKeys) > 0 {
			*selectedIndex = (*selectedIndex - 1 + len(*filteredKeys)) % len(*filteredKeys)
		}
		g.Update(func(g *gocui.Gui) error { return nil })
		return nil
	})

	// クエリ文字列の文字削除
	g.SetKeybinding("", gocui.KeyCtrlH, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(*searchQuery) > 0 {
			*searchQuery = (*searchQuery)[:len(*searchQuery)-1]
		}
		*filteredKeys = updateSelectedIndex(searchQuery, jp.keys, selectedIndex)
		g.Update(func(g *gocui.Gui) error { return nil })
		return nil
	})

	// 候補を選択してクエリに反映 (Enter)
	g.SetKeybinding("", gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(*filteredKeys) > 0 {
			*searchQuery = (*filteredKeys)[*selectedIndex]
		}
		g.Update(func(g *gocui.Gui) error { return nil })
		return nil
	})

	// 文字入力でクエリを編集
	for char := rune('a'); char <= rune('z'); char++ {
		char := char
		g.SetKeybinding("", char, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			*searchQuery += string(char)
			*filteredKeys = updateSelectedIndex(searchQuery, jp.keys, selectedIndex)
			g.Update(func(g *gocui.Gui) error { return nil })
			return nil
		})
	}
	for char := rune('0'); char <= rune('9'); char++ {
		char := char
		g.SetKeybinding("", char, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			*searchQuery += string(char)
			*filteredKeys = updateSelectedIndex(searchQuery, jp.keys, selectedIndex)
			g.Update(func(g *gocui.Gui) error { return nil })
			return nil
		})
	}

	// 特殊文字の入力
	specialChars := []rune{'[', ']', '.', '_'}
	for _, char := range specialChars {
		char := char
		g.SetKeybinding("", char, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			*searchQuery += string(char)
			*filteredKeys = updateSelectedIndex(searchQuery, jp.keys, selectedIndex)
			g.Update(func(g *gocui.Gui) error { return nil })
			return nil
		})
	}
}

// クエリ文字列に基づいて候補を更新
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
		// 配列フィールドの場合は全要素を収集
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

	// 通常のクエリ処理
	result := gjson.GetBytes(jsonData, query)
	if result.Exists() {
		if result.IsObject() || result.IsArray() {
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, []byte(result.Raw), "", "  "); err == nil {
				return prettyJSON.String()
			}
			return result.Raw
		}
		// 文字列やその他の型
		return result.String()
	}

	return "Query failed. No matching data found."
}
