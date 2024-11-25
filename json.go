package main

import (
	"fmt"
	"strings"

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

	walk("", gjson.ParseBytes(jp.jsonData))

	// remove invalid keys
	jp.keys = filterInvalidKeys(jp.keys)
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

// remove invalid keys (e.g. "foo[]")
func filterInvalidKeys(keys []string) []string {
	var validKeys []string
	for _, key := range keys {
		if !strings.HasSuffix(key, "[]") { // remove array with '[]' suffix
			validKeys = append(validKeys, key)
		}
	}
	return validKeys
}
