package version

import (
	"os"
	"strings"
	"sync"
)

var (
	cached   string
	once     sync.Once
	readFile = os.ReadFile
)

func Version() string {
	once.Do(func() {
		cached, _ = readVersionFile(".version")
	})
	return cached
}

func readVersionFile(path string) (string, error) {
	data, err := readFile(path)
	if err != nil {
		return "unknown", err
	}
	return strings.TrimSpace(string(data)), nil
}
