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
func (jp *JSONProcessor) extractKeys() {
	seenKeys := make(map[string]struct{})
	var walk func(prefix string, value gjson.Result)
	walk = func(prefix string, value gjson.Result) {
		if value.IsObject() {
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
		} else if value.IsArray() {
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
		}
	}

	walk("", gjson.ParseBytes(jp.jsonData))

	// remove invalid keys
	jp.keys = filterInvalidKeys(jp.keys)
}

// remove invalid keys (e.g. "foo[]")
func filterInvalidKeys(keys []string) []string {
	var validKeys []string
	for _, key := range keys {
		if !strings.HasSuffix(key, "[]") { // remove keys at last '[]'
			validKeys = append(validKeys, key)
		}
	}
	return validKeys
}
