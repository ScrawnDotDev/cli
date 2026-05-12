package ui

import (
	"fmt"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerTask runs fn with an animated spinner next to message.
// Prints ✓ message on success, ✗ on error. Returns the error.
func SpinnerTask(message string, fn func() error) error {
	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Printf("\r%s %s", spinnerFrames[i%len(spinnerFrames)], message)
				i++
			}
		}
	}()

	err := fn()
	time.Sleep(100 * time.Millisecond) // let spinner goroutine exit

	if err != nil {
		fmt.Printf("\r%s %s\n", failureStyle.Render("✗"), message)
		return err
	}
	fmt.Printf("\r%s %s\n", successStyle.Render("✓"), message)
	return nil
}
