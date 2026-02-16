package doctor

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"unicode"
)

type Check func() error
var registry = make(map[string]Check)

func Register(provider string, check Check) {
	registry[provider] = check
}

func Run(provider string) error {
	check, ok := registry[provider]
	if !ok {
		// If no doctor exists for "massive", we just return nil and let q handle it
		return nil 
	}
	return check()
}

func AskConfirm(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [Y/n]: ", prompt)
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	return input == "" || input == "y" || input == "yes"
}

func GetBaseProvider(name string) string {
	return strings.TrimFunc(name, func(r rune) bool {
		return unicode.IsDigit(r)
	})
}