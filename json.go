package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/quick"
	"github.com/tidwall/gjson"
)

// JSONProcessor handles extraction and querying of JSON data
// It maintains the raw JSON bytes and extracted key paths
type JSONProcessor struct {
	jsonData []byte   // Raw JSON data
	keys     []string // Extracted key paths from the JSON structure
}

// extractKeys traverses the JSON structure and extracts all accessible key paths
// Uses a recursive walk function to handle nested objects and arrays
// Filters out invalid keys and prevents duplicate entries
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

// processObject recursively processes JSON objects and extracts all nested key paths
// Prevents duplicate keys using the seenKeys map and maintains proper path prefixes
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

// processArray recursively processes JSON arrays and extracts indexed and wildcard key paths
// Generates both specific indices (e.g., foo[0]) and array patterns (e.g., foo.# and foo[].bar)
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

// filterInvalidKeys removes keys that don't match valid patterns or root keys
// Filters out keys ending with "[]" or containing "[]" in the middle
// Also validates that keys have a proper root when rootKeys are provided
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

// getParsedResult retrieves and formats the JSON value for a given query path
// Handles different query types: array patterns (with []), indexed access, and simple paths
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
	gjsonQuery := strings.ReplaceAll(query, "[", ".")
	gjsonQuery = strings.ReplaceAll(gjsonQuery, "]", "")

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

// Utility Functions

// fuzzyFind performs fuzzy matching to check if all characters in searchQuery
// appear in the key string in the same order (not necessarily consecutively)
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

// highlightJSON applies syntax highlighting to JSON strings using Chroma
// Returns the original string if highlighting fails
func highlightJSON(jsonData string) string {
	var highlighted bytes.Buffer
	err := quick.Highlight(&highlighted, jsonData, "json", "terminal", "monokai")
	if err != nil {
		return jsonData
	}
	return highlighted.String()
}
