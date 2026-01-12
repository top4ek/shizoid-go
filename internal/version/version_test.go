package version

import (
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func resetState() {
	readFile = os.ReadFile
	once = sync.Once{}
	cached = ""
}

func TestVersion_FileExists(t *testing.T) {
	defer resetState()

	readFile = func(path string) ([]byte, error) {
		assert.Equal(t, ".version", path)
		return []byte("1.2.3\n"), nil
	}

	v := Version()

	assert.Equal(t, "1.2.3", v)
}

func TestVersion_FileMissing(t *testing.T) {
	defer resetState()

	readFile = func(string) ([]byte, error) {
		return nil, errors.New("file not found")
	}

	v := Version()

	assert.Equal(t, "unknown", v)
}

func TestVersion_IsCached(t *testing.T) {
	defer resetState()

	calls := 0
	readFile = func(string) ([]byte, error) {
		calls++
		return []byte("1.0.0"), nil
	}

	first := Version()
	second := Version()

	assert.Equal(t, "1.0.0", first)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, calls, "readFile should be called only once")
}

func TestVersion_TrimsWhitespace(t *testing.T) {
	defer resetState()

	readFile = func(string) ([]byte, error) {
		return []byte("  2.3.4 \n"), nil
	}

	v := Version()

	assert.Equal(t, "2.3.4", v)
}
