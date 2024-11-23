package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/fatih/color"
	"github.com/ktr0731/go-fuzzyfinder"
	"github.com/tidwall/gjson"
)

type JSONProcessor struct {
	filePath string
	jsonData []byte
	keys     []string
}

func (jp *JSONProcessor) readFile() error {
	data, err := os.ReadFile(jp.filePath)
	if err != nil {
		return fmt.Errorf("Error reading file: %v", err)
	}
	jp.jsonData = data
	return nil
}

func (jp *JSONProcessor) extractKeys() {
	jp.keys = uniqueKeys(filterKeys(extractKeys(string(jp.jsonData))))
	jp.keys = append([]string{""}, jp.keys...) // set null string as first element
}

func (jp *JSONProcessor) startFuzzyFinder() (string, error) {
	idx, err := fuzzyfinder.Find(jp.keys, func(i int) string {
		return jp.keys[i]
	}, fuzzyfinder.WithPreviewWindow(func(i int, w, h int) string {
		if i == -1 {
			return "No selection"
		}
		query := jp.keys[i]
		if query == "" {
			return "Query: (Full JSON)\n\n[Parsed Result]:\n" + colorizeJSON(getParsedResult(query, jp.jsonData))
		}
		return fmt.Sprintf("Query: %s\n\n[Parsed Result]:\n%s", query, colorizeJSON(getParsedResult(query, jp.jsonData)))
	}))
	if err != nil {
		return "", fmt.Errorf("No selection made")
	}
	return jp.keys[idx], nil
}

func main() {
	var jsonStr strings.Builder

	if len(os.Args) > 1 {
		filePath := os.Args[1]
		jp := &JSONProcessor{
			filePath: filePath,
		}
		if err := jp.readFile(); err != nil {
			fmt.Println(err)
			return
		}
		jsonStr.Write(jp.jsonData)
	} else {
		// input from pipe
		stat, err := os.Stdin.Stat()
		if err != nil {
			fmt.Println("Error stating stdin:", err)
			return
		}

		if (stat.Mode() & os.ModeCharDevice) == 0 {
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				jsonStr.WriteString(scanner.Text() + "\n")
			}
			if err := scanner.Err(); err != nil {
				fmt.Println("Error reading stdin:", err)
				return
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

	for {
		query, err := jp.startFuzzyFinder()
		if err != nil {
			fmt.Println("Error in fuzzy finder:", err)
			return
		}
		if query == "" {
			break
		}

		result := getParsedResult(query, jp.jsonData)
		fmt.Println(result)
	}
}

// uniqueKeys is a function that removes duplicates from a key list
func uniqueKeys(keys []string) []string {
	seen := make(map[string]struct{})
	var result []string
	for _, key := range keys {
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			result = append(result, key)
		}
	}
	return result
}

// remove inappropriate queries (e.g. "contributors[]" or "metadata.tags[]") from candidates
func filterKeys(keys []string) []string {
	var filteredKeys []string
	for _, key := range keys {
		// remove queries with array suffix
		if strings.HasSuffix(key, "[]") {
			continue
		}
		filteredKeys = append(filteredKeys, key)
	}
	return filteredKeys
}

// extractKeys is a recursive function that extracts all keys in JSON
func extractKeys(jsonStr string) []string {
	var keys []string
	var walk func(prefix string, value gjson.Result)

	walk = func(prefix string, value gjson.Result) {
		if value.IsObject() {
			if prefix != "" {
				keys = append(keys, prefix)
			}
			value.ForEach(func(key, val gjson.Result) bool {
				fullKey := key.String()
				if prefix != "" {
					fullKey = prefix + "." + key.String()
				}
				keys = append(keys, fullKey)
				walk(fullKey, val)
				return true
			})
		} else if value.IsArray() {
			if prefix != "" {
				keys = append(keys, prefix+"[]")
			}
			value.ForEach(func(_, val gjson.Result) bool {
				walk(prefix+"[]", val)
				return true
			})
		}
	}

	result := gjson.Parse(jsonStr)
	walk("", result)
	return keys
}

// getParsedResult is a function that parses and retrieves the result of the query
func getParsedResult(query string, jsonData []byte) string {
	if query == "" {
		// if null string is selected, return the whole JSON
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, jsonData, "", "  "); err == nil {
			return prettyJSON.String()
		}
		return string(jsonData)
	}

	result := gjson.GetBytes(jsonData, query)
	if result.Exists() {
		// check json type
		if result.IsObject() || result.IsArray() {
			// if the result is an object or an array, pretty print it
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, []byte(result.Raw), "", "  "); err == nil {
				return prettyJSON.String()
			}
			return result.Raw
		} else if result.Type == gjson.String {
			// ordinary string
			return result.String()
		} else if result.Type == gjson.Number {
			// numeric
			return fmt.Sprintf("%v", result.Float())
		} else if result.Type == gjson.True || result.Type == gjson.False {
			// bool
			return fmt.Sprintf("%v", result.Bool())
		} else if result.Type == gjson.Null {
			// null
			return "null"
		}
	}

	// check if the query is an array field (e.g. "contributors[]", "metadata.tags[]")
	if strings.Contains(query, "[]") {
		baseKey := strings.Split(query, "[]")[0]
		field := strings.Split(query, "[]")[1]
		if field != "" {
			field = strings.TrimPrefix(field, ".")
			arrayResult := gjson.GetBytes(jsonData, baseKey)
			if arrayResult.IsArray() {
				var items []string
				arrayResult.ForEach(func(_, val gjson.Result) bool {
					fieldValue := val.Get(field)
					if fieldValue.Exists() {
						items = append(items, fieldValue.String())
					}
					return true
				})
				if len(items) > 0 {
					return "[" + strings.Join(items, ", ") + "]"
				}
			}
		}
	}

	// Query failed. No matching data found.
	return "Query failed. No matching data found."
}

// colroizeJSON is a function that colorizes JSON string
func colorizeJSON(jsonStr string) string {
	keyColor := color.New(color.FgCyan).SprintFunc()
	stringColor := color.New(color.FgGreen).SprintFunc()
	numberColor := color.New(color.FgYellow).SprintFunc()
	boolColor := color.New(color.FgMagenta).SprintFunc()
	nullColor := color.New(color.FgHiBlack).SprintFunc()

	var result strings.Builder
	re := regexp.MustCompile(`"(.*?)"\s*:\s*("(.*?)"|true|false|null|[\d.]+)`)
	for _, line := range strings.Split(jsonStr, "\n") {
		matches := re.FindAllStringSubmatch(line, -1)
		if matches == nil {
			result.WriteString(line + "\n")
			continue
		}
		for _, match := range matches {
			key := match[1]
			value := match[2]
			var coloredValue string
			if strings.HasPrefix(value, `"`) {
				coloredValue = stringColor(value)
			} else if value == "true" || value == "false" {
				coloredValue = boolColor(value)
			} else if value == "null" {
				coloredValue = nullColor(value)
			} else {
				coloredValue = numberColor(value)
			}
			line = strings.Replace(line, match[0], fmt.Sprintf(`"%s": %s`, keyColor(key), coloredValue), 1)
		}
		result.WriteString(line + "\n")
	}
	return result.String()
}
