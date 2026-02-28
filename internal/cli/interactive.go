package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type TurnRunner func(input string) (string, error)

func RunInteractive(sessionID string, runTurn TurnRunner) error {
	if strings.TrimSpace(sessionID) != "" {
		fmt.Fprintf(os.Stderr, "session: %s\n", sessionID)
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("you> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		handled, shouldExit := dispatchLocalCommand(input)
		if handled {
			if shouldExit {
				break
			}
			continue
		}

		responseText, runErr := runTurn(input)
		if runErr != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", runErr)
			continue
		}

		fmt.Printf("assistant> %s\n", responseText)
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return scanErr
	}

	return nil
}

func dispatchLocalCommand(input string) (handled bool, shouldExit bool) {
	switch input {
	case ":exit", ":quit":
		return true, true
	default:
		return false, false
	}
}
