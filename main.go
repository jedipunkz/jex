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
			if prefix != "" {
				jp.keys = append(jp.keys, prefix+"[]")
			}
		}
	}
	walk("", gjson.ParseBytes(jp.jsonData))
}

func runTUI(jp *JSONProcessor) {
	var searchQuery string
	var selectedIndex int

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		panic(err)
	}
	defer g.Close()

	g.SetManagerFunc(func(g *gocui.Gui) error {
		// クエリ入力ビュー
		vQuery, err := g.SetView("query", 0, 0, 50, 3)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		vQuery.Clear()
		fmt.Fprintf(vQuery, "Search Query: %s", searchQuery)

		// 候補表示ビュー
		vCandidates, err := g.SetView("candidates", 0, 4, 50, 10)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		vCandidates.Clear()
		for i, key := range jp.keys {
			if i == selectedIndex {
				fmt.Fprintf(vCandidates, "> %s\n", key)
			} else {
				fmt.Fprintf(vCandidates, "  %s\n", key)
			}
		}

		// JSON 表示ビュー
		vJSON, err := g.SetView("json", 0, 11, 50, 20)
		if err != nil && err != gocui.ErrUnknownView {
			return err
		}
		vJSON.Clear()
		displayParsedResult(vJSON, searchQuery, jp.jsonData)

		return nil
	})

	// キーバインド設定
	setKeybindings(g, jp, &searchQuery, &selectedIndex)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		fmt.Println("Error:", err)
	}
}

func setKeybindings(g *gocui.Gui, jp *JSONProcessor, searchQuery *string, selectedIndex *int) {
	// Quit
	g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	})

	// 候補選択を Ctrl+N / Ctrl+P で操作
	g.SetKeybinding("", gocui.KeyCtrlN, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		*selectedIndex = (*selectedIndex + 1) % len(jp.keys)
		return nil
	})
	g.SetKeybinding("", gocui.KeyCtrlP, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		*selectedIndex = (*selectedIndex - 1 + len(jp.keys)) % len(jp.keys)
		return nil
	})

	// クエリ文字列の文字削除
	g.SetKeybinding("", gocui.KeyCtrlH, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(*searchQuery) > 0 {
			*searchQuery = (*searchQuery)[:len(*searchQuery)-1]
		}
		updateSelectedIndex(searchQuery, jp.keys, selectedIndex)
		return nil
	})

	// 候補を選択してクエリに反映 (Enter)
	g.SetKeybinding("", gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if len(jp.keys) > 0 {
			*searchQuery = jp.keys[*selectedIndex]
		}
		return nil
	})

	// 文字入力でクエリを編集
	for char := rune('a'); char <= rune('z'); char++ {
		char := char
		g.SetKeybinding("", char, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			*searchQuery += string(char)
			updateSelectedIndex(searchQuery, jp.keys, selectedIndex)
			return nil
		})
	}
	for char := rune('0'); char <= rune('9'); char++ {
		char := char
		g.SetKeybinding("", char, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
			*searchQuery += string(char)
			updateSelectedIndex(searchQuery, jp.keys, selectedIndex)
			return nil
		})
	}
}

// クエリ文字列に基づいて候補を更新
func updateSelectedIndex(searchQuery *string, keys []string, selectedIndex *int) {
	for i, key := range keys {
		if strings.HasPrefix(key, *searchQuery) {
			*selectedIndex = i
			return
		}
	}
	// 一致する候補がなければ最初の候補を選択
	*selectedIndex = 0
}

func displayParsedResult(v *gocui.View, query string, jsonData []byte) {
	result := getParsedResult(query, jsonData)
	fmt.Fprintln(v, result)
}

func getParsedResult(query string, jsonData []byte) string {
	if query == "" {
		// 全 JSON 表示
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, jsonData, "", "  "); err == nil {
			return prettyJSON.String()
		}
		return string(jsonData)
	}

	result := gjson.GetBytes(jsonData, query)
	if result.Exists() {
		// オブジェクトや配列の場合は整形
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
