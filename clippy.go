package main

import (
	"fmt"
	"golang.design/x/clipboard"
)

// Prompt
func prompt(content string) {
	fmt.Println(content)
	err := clipboard.Init()
	if err == nil {
		clipboard.Write(clipboard.FmtText, []byte(content))
	}

}
