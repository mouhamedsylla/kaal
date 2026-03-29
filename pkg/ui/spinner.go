package ui

import (
	"fmt"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner runs fn in the background while showing an animated spinner.
// It prints a success or error message when fn returns.
func Spinner(label string, fn func() error) error {
	done := make(chan error, 1)
	go func() { done <- fn() }()

	i := 0
	for {
		select {
		case err := <-done:
			fmt.Printf("\r\033[K") // clear spinner line
			if err != nil {
				Error(label + ": " + err.Error())
				return err
			}
			Success(label)
			return nil
		default:
			frame := spinnerFrames[i%len(spinnerFrames)]
			fmt.Printf("\r%s %s", frame, label)
			time.Sleep(80 * time.Millisecond)
			i++
		}
	}
}
