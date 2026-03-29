package handlers

import (
	"io"
	"os"
	"strings"
)

// captureOutput runs fn while redirecting os.Stdout to a buffer and returns
// the captured text. This is required for MCP handlers that call internal
// packages which print via fmt.Println / ui.* — those writes must not reach
// the JSON-RPC stdio transport.
func captureOutput(fn func() error) (string, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}

	old := os.Stdout
	os.Stdout = w

	var buf strings.Builder
	done := make(chan struct{})
	go func() {
		io.Copy(&buf, r)
		close(done)
	}()

	fnErr := fn()
	w.Close()
	<-done
	os.Stdout = old
	r.Close()

	return buf.String(), fnErr
}
