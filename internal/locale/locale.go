package locale

import (
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"

	"embed"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"shizoid/internal/logger"
)

//go:embed locales/*.yaml
var localesFS embed.FS

var (
	loadOnce  sync.Once
	data      = map[string]map[string]any{}
	available []string
)

func ensureLoaded() {
	loadOnce.Do(func() {
		entries, err := localesFS.ReadDir("locales")
		if err != nil {
			logger.Instance().Error("locale read dir", zap.Error(err))
			return
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
				continue
			}
			raw, err := localesFS.ReadFile("locales/" + e.Name())
			if err != nil {
				logger.Instance().Error("locale read file", zap.String("file", e.Name()), zap.Error(err))
				continue
			}
			var root map[string]any
			if err := yaml.Unmarshal(raw, &root); err != nil {
				logger.Instance().Error("locale parse", zap.String("file", e.Name()), zap.Error(err))
				continue
			}
			for lang, v := range root {
				if m, ok := v.(map[string]any); ok {
					data[lang] = m
				}
			}
		}
		for lang := range data {
			available = append(available, lang)
		}
		sort.Strings(available)
		if len(available) == 0 {
			logger.Instance().Error("locale load", zap.Error(fmt.Errorf("no locales loaded")))
		}
	})
}

// Available returns the sorted list of available locale codes.
func Available() []string {
	ensureLoaded()
	return available
}

// Has reports whether the given locale code exists.
func Has(lang string) bool {
	ensureLoaded()
	_, ok := data[lang]
	return ok
}

func lookup(lang, key string) (any, bool) {
	ensureLoaded()
	m, ok := data[lang]
	if !ok {
		return nil, false
	}
	var cur any = m
	for _, part := range strings.Split(key, ".") {
		mm, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = mm[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// T returns the translated, interpolated string for the dotted key.
func T(lang, key string, vars ...any) string {
	v, ok := lookup(lang, key)
	if !ok {
		return key
	}
	s, ok := v.(string)
	if !ok {
		return key
	}
	return interpolate(s, toVarMap(vars))
}

// List returns the string slice for the dotted key (nil if absent).
func List(lang, key string) []string {
	v, ok := lookup(lang, key)
	if !ok {
		return nil
	}
	sl, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(sl))
	for _, e := range sl {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// Symbol is an emoji paired with a localized word shown in the captcha prompt.
type Symbol struct {
	Emoji string
	Word  string
}

// Symbols returns structured emoji/word pairs at the dotted key (nil if absent).
func Symbols(lang, key string) []Symbol {
	v, ok := lookup(lang, key)
	if !ok {
		return nil
	}
	sl, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]Symbol, 0, len(sl))
	for _, e := range sl {
		m, ok := e.(map[string]any)
		if !ok {
			continue
		}
		emoji, _ := m["emoji"].(string)
		word, _ := m["word"].(string)
		if emoji == "" || word == "" {
			continue
		}
		out = append(out, Symbol{Emoji: emoji, Word: word})
	}
	return out
}

// Random returns a random element from the list at key, or "" if empty.
func Random(lang, key string) string {
	list := List(lang, key)
	if len(list) == 0 {
		return ""
	}
	return list[rand.IntN(len(list))]
}

func toVarMap(vars []any) map[string]any {
	if len(vars) == 1 {
		if m, ok := vars[0].(map[string]any); ok {
			return m
		}
	}
	m := make(map[string]any, len(vars)/2)
	for i := 0; i+1 < len(vars); i += 2 {
		key, ok := vars[i].(string)
		if !ok {
			continue
		}
		m[key] = vars[i+1]
	}
	return m
}

func interpolate(s string, vars map[string]any) string {
	if len(vars) == 0 || !strings.Contains(s, "%{") {
		return s
	}
	var b strings.Builder
	for {
		start := strings.Index(s, "%{")
		if start < 0 {
			b.WriteString(s)
			break
		}
		end := strings.Index(s[start:], "}")
		if end < 0 {
			b.WriteString(s)
			break
		}
		end += start
		b.WriteString(s[:start])
		name := s[start+2 : end]
		if val, ok := vars[name]; ok {
			b.WriteString(fmt.Sprint(val))
		} else {
			b.WriteString(s[start : end+1])
		}
		s = s[end+1:]
	}
	return b.String()
}
