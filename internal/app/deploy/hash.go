package deploy

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
)

// computeHash returns the hex-encoded SHA-256 of the concatenated contents
// of all readable files in paths. Missing files are silently skipped.
func computeHash(paths []string) (string, error) {
	h := sha256.New()
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		if _, err := io.Copy(h, f); err != nil {
			f.Close()
			return "", err
		}
		f.Close()
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
