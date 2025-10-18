package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/quick"
	"github.com/tidwall/gjson"
)

type JSONProcessor struct {
	jsonData []byte
	keys     []string
}

// JSONProcessor extract keys from JSON data
// seenKeys is used to prevent duplicate keys
// walk is a recursive function to walk through JSON data
func (jp *JSONProcessor) extractKeys() {
	seenKeys := make(map[string]struct{})
	var walk func(prefix string, value gjson.Result)
	walk = func(prefix string, value gjson.Result) {
		if value.IsObject() {
			jp.processObject(prefix, value, seenKeys, walk)
		} else if value.IsArray() {
			jp.processArray(prefix, value, seenKeys, walk)
		}
	}

	parsed := gjson.ParseBytes(jp.jsonData)
	walk("", parsed)

	// Get root keys for validation
	var rootKeys []string
	if parsed.IsObject() {
		parsed.ForEach(func(key, _ gjson.Result) bool {
			rootKeys = append(rootKeys, key.String())
			return true
		})
	}

	// remove invalid keys
	jp.keys = filterInvalidKeys(jp.keys, rootKeys)
}

// processObject processes JSON objects and extracts keys
func (jp *JSONProcessor) processObject(prefix string, value gjson.Result, seenKeys map[string]struct{}, walk func(string, gjson.Result)) {
	value.ForEach(func(key, val gjson.Result) bool {
		fullKey := key.String()
		if prefix != "" {
			fullKey = prefix + "." + fullKey
		}
		if _, exists := seenKeys[fullKey]; !exists {
			seenKeys[fullKey] = struct{}{}
			jp.keys = append(jp.keys, fullKey)
		}
		walk(fullKey, val)
		return true
	})
}

// processArray processes JSON arrays and extracts keys
func (jp *JSONProcessor) processArray(prefix string, value gjson.Result, seenKeys map[string]struct{}, walk func(string, gjson.Result)) {
	arrayKey := fmt.Sprintf("%s.#", prefix)
	if _, exists := seenKeys[arrayKey]; !exists {
		seenKeys[arrayKey] = struct{}{}
		jp.keys = append(jp.keys, arrayKey)
	}
	value.ForEach(func(index, val gjson.Result) bool {
		elementKey := fmt.Sprintf("%s[%d]", prefix, index.Int())
		if _, exists := seenKeys[elementKey]; !exists {
			seenKeys[elementKey] = struct{}{}
			jp.keys = append(jp.keys, elementKey)
		}
		walk(elementKey, val)
		return true
	})
	// add array element keys (e.g. foo[].name)
	if prefix != "" {
		value.ForEach(func(_, val gjson.Result) bool {
			val.ForEach(func(key, val gjson.Result) bool {
				fullKey := fmt.Sprintf("%s[].%s", prefix, key.String())
				fullKey = strings.TrimSuffix(fullKey, ".")
				if _, exists := seenKeys[fullKey]; !exists {
					seenKeys[fullKey] = struct{}{}
					jp.keys = append(jp.keys, fullKey)
				}
				// add nested array element keys (e.g. foo[].bar[0])
				if val.IsArray() {
					val.ForEach(func(index, nestedVal gjson.Result) bool {
						nestedKey := fmt.Sprintf("%s[%d]", fullKey, index.Int())
						if _, exists := seenKeys[nestedKey]; !exists {
							seenKeys[nestedKey] = struct{}{}
							jp.keys = append(jp.keys, nestedKey)
						}
						return true
					})
				}
				return true
			})
			return false // only process the first element
		})
	}
}

func filterInvalidKeys(keys []string, rootKeys []string) []string {
	var validKeys []string
	for _, key := range keys {
		// Remove keys ending with "[]"
		if strings.HasSuffix(key, "[]") {
			continue
		}

		if strings.Contains(key, "[]") {
			continue
		}

		if len(rootKeys) > 0 {
			validRoot := false
			for _, rootKey := range rootKeys {
				if key == rootKey || strings.HasPrefix(key, rootKey+".") || strings.HasPrefix(key, rootKey+"[") {
					validRoot = true
					break
				}
			}
			if !validRoot {
				continue
			}
		}

		validKeys = append(validKeys, key)
	}
	return validKeys
}

// JSON Query and Extraction Functions

// getParsedResult gets the parsed result for a given query
func getParsedResult(query string, jsonData []byte) string {
	if strings.Contains(query, "[]") {
		return handleArrayQuery(query, jsonData)
	}

	if strings.Contains(query, "[") && strings.Contains(query, "]") {
		return handleIndexedQuery(query, jsonData)
	}

	return handleOrdinaryQuery(query, jsonData)
}

// handleArrayQuery handles queries with [] pattern
func handleArrayQuery(query string, jsonData []byte) string {
	baseQuery := strings.Split(query, "[]")[0]
	field := strings.TrimPrefix(strings.Split(query, "[]")[1], ".")
	arrayResult := gjson.GetBytes(jsonData, baseQuery)
	if arrayResult.IsArray() {
		var values []string
		arrayResult.ForEach(func(_, val gjson.Result) bool {
			if strings.Contains(field, "[") && strings.Contains(field, "]") {
				values = append(values, handleNestedArray(val, field)...)
			} else if field == "" {
				// Return entire array element
				if val.IsObject() || val.IsArray() {
					var prettyJSON bytes.Buffer
					if err := json.Indent(&prettyJSON, []byte(val.Raw), "", "  "); err == nil {
						values = append(values, prettyJSON.String())
					} else {
						values = append(values, val.Raw)
					}
				} else {
					values = append(values, val.String())
				}
			} else {
				result := val.Get(field)
				if result.IsObject() || result.IsArray() {
					var prettyJSON bytes.Buffer
					if err := json.Indent(&prettyJSON, []byte(result.Raw), "", "  "); err == nil {
						values = append(values, prettyJSON.String())
					} else {
						values = append(values, result.Raw)
					}
				} else {
					values = append(values, result.String())
				}
			}
			return true
		})
		return strings.Join(values, "\n")
	}
	return "Query failed. No matching data found."
}

// handleNestedArray handles nested array queries
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
					result := nestedVal.Get(nestedField)
					if result.IsObject() || result.IsArray() {
						var prettyJSON bytes.Buffer
						if err := json.Indent(&prettyJSON, []byte(result.Raw), "", "  "); err == nil {
							values = append(values, prettyJSON.String())
						} else {
							values = append(values, result.Raw)
						}
					} else {
						values = append(values, result.String())
					}
				} else {
					if nestedVal.IsObject() || nestedVal.IsArray() {
						var prettyJSON bytes.Buffer
						if err := json.Indent(&prettyJSON, []byte(nestedVal.Raw), "", "  "); err == nil {
							values = append(values, prettyJSON.String())
						} else {
							values = append(values, nestedVal.Raw)
						}
					} else {
						values = append(values, nestedVal.String())
					}
				}
			}
			return true
		})
	}
	return values
}

// handleIndexedQuery handles queries with array indices like [0]
func handleIndexedQuery(query string, jsonData []byte) string {
	// Convert query from "company.departments[0].teams[0].members[0]"
	// to gjson format "company.departments.0.teams.0.members.0"
	gjsonQuery := strings.Replace(query, "[", ".", -1)
	gjsonQuery = strings.Replace(gjsonQuery, "]", "", -1)

	result := gjson.GetBytes(jsonData, gjsonQuery)
	if result.Exists() {
		// Pretty-print objects and arrays
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

// handleOrdinaryQuery handles simple queries without arrays
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

// splitQuery splits a query into base and field parts
func splitQuery(query string) (string, string) {
	if dotIndex := strings.Index(query, "."); dotIndex != -1 {
		return query[:dotIndex], query[dotIndex+1:]
	}
	return query, ""
}

// Utility Functions

// fuzzyFind checks if all characters in searchQuery are in key in order
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

// highlightJSON applies syntax highlighting to JSON
func highlightJSON(jsonData string) string {
	var highlighted bytes.Buffer
	err := quick.Highlight(&highlighted, jsonData, "json", "terminal", "monokai")
	if err != nil {
		return jsonData
	}
	return highlighted.String()
}
