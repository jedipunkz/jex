package main

import (
	"strings"

	"github.com/tidwall/gjson"
)

type JSONProcessor struct {
	jsonData []byte
	keys     []string
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
