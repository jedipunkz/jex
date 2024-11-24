package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

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
