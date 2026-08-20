package telegram

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-telegram/bot"
)

var errInvalidMarkdown = errors.New("invalid markdown v2")

const mdV2Special = "_*[]()~`>#+-=|{}.!"

func isMDSpecial(r rune) bool {
	return strings.ContainsRune(mdV2Special, r)
}

// ValidateV2 reports whether s is valid Telegram MarkdownV2 message text.
func ValidateV2(s string) error {
	_, err := parseV2(s, false)
	return err
}

// SanitizeV2 returns valid MarkdownV2 text, preserving valid markup and escaping the rest.
func SanitizeV2(s string) string {
	out, err := parseV2(s, true)
	if err != nil {
		return bot.EscapeMarkdown(s)
	}
	if err := ValidateV2(out); err != nil {
		return bot.EscapeMarkdown(s)
	}
	return out
}

// FormatPlain escapes plain text for MarkdownV2.
func FormatPlain(s string) string {
	return bot.EscapeMarkdown(s)
}

func parseV2(s string, sanitize bool) (string, error) {
	p := &mdParser{rs: []rune(s), sanitize: sanitize}
	if err := p.parsePlain(); err != nil {
		return "", err
	}
	return p.out.String(), nil
}

type mdParser struct {
	rs       []rune
	i        int
	sanitize bool
	out      strings.Builder
}

func (p *mdParser) parsePlain() error {
	for p.i < len(p.rs) {
		if p.rs[p.i] == '\\' {
			if p.i+1 >= len(p.rs) {
				if p.sanitize {
					p.out.WriteString(`\\`)
					p.i++
					return nil
				}
				return fmt.Errorf("%w: trailing backslash", errInvalidMarkdown)
			}
			p.out.WriteRune('\\')
			p.out.WriteRune(p.rs[p.i+1])
			p.i += 2
			continue
		}
		if consumed, ok := p.tryEntity(); ok {
			p.out.WriteString(consumed)
			continue
		}
		r := p.rs[p.i]
		if isMDSpecial(r) {
			if p.sanitize {
				p.out.WriteRune('\\')
			} else {
				return fmt.Errorf("%w: unescaped %q at %d", errInvalidMarkdown, r, p.i)
			}
		}
		p.out.WriteRune(r)
		p.i++
	}
	return nil
}

func (p *mdParser) tryEntity() (string, bool) {
	start := p.i
	if p.i+2 < len(p.rs) && p.rs[p.i] == '|' && p.rs[p.i+1] == '|' {
		if body, ok := p.parseDelimited("||", "||"); ok {
			return "||" + body + "||", true
		}
		p.i = start
	}
	if p.i+2 < len(p.rs) && p.rs[p.i] == '_' && p.rs[p.i+1] == '_' {
		if body, ok := p.parseDelimited("__", "__"); ok {
			return "__" + body + "__", true
		}
		p.i = start
	}
	if p.i+2 < len(p.rs) && p.rs[p.i] == '`' && p.rs[p.i+1] == '`' && p.rs[p.i+2] == '`' {
		if body, ok := p.parsePre(); ok {
			return "```" + body + "```", true
		}
		p.i = start
	}
	switch p.rs[p.i] {
	case '*':
		if body, ok := p.parseDelimited("*", "*"); ok {
			return "*" + body + "*", true
		}
	case '_':
		if body, ok := p.parseDelimited("_", "_"); ok {
			return "_" + body + "_", true
		}
	case '~':
		if body, ok := p.parseDelimited("~", "~"); ok {
			return "~" + body + "~", true
		}
	case '`':
		if body, ok := p.parseDelimited("`", "`"); ok {
			return "`" + body + "`", true
		}
	case '[':
		if link, ok := p.parseLink(); ok {
			return link, true
		}
	}
	p.i = start
	return "", false
}

func (p *mdParser) parseDelimited(open, close string) (string, bool) {
	openRunes := []rune(open)
	closeRunes := []rune(close)
	if p.i+len(openRunes) > len(p.rs) {
		return "", false
	}
	for j, r := range openRunes {
		if p.rs[p.i+j] != r {
			return "", false
		}
	}
	contentStart := p.i + len(openRunes)
	search := p.i + len(openRunes)
	for search < len(p.rs) {
		if p.rs[search] == '\\' {
			search += 2
			continue
		}
		if search+len(closeRunes) <= len(p.rs) {
			match := true
			for j, r := range closeRunes {
				if p.rs[search+j] != r {
					match = false
					break
				}
			}
			if match {
				body := string(p.rs[contentStart:search])
				sanitized, err := parseV2(body, p.sanitize)
				if err != nil {
					return "", false
				}
				p.i = search + len(closeRunes)
				return sanitized, true
			}
		}
		search++
	}
	return "", false
}

func (p *mdParser) parsePre() (string, bool) {
	start := p.i + 3
	search := start
	for search+2 < len(p.rs) {
		if p.rs[search] == '\\' {
			search += 2
			continue
		}
		if p.rs[search] == '`' && p.rs[search+1] == '`' && p.rs[search+2] == '`' {
			body := string(p.rs[start:search])
			p.i = search + 3
			return body, true
		}
		search++
	}
	return "", false
}

func (p *mdParser) parseLink() (string, bool) {
	start := p.i
	p.i++
	textStart := p.i
	for p.i < len(p.rs) {
		if p.rs[p.i] == '\\' && p.i+1 < len(p.rs) {
			p.i += 2
			continue
		}
		if p.rs[p.i] == ']' {
			text := string(p.rs[textStart:p.i])
			p.i++
			if p.i >= len(p.rs) || p.rs[p.i] != '(' {
				p.i = start
				return "", false
			}
			p.i++
			urlStart := p.i
			for p.i < len(p.rs) {
				if p.rs[p.i] == '\\' && p.i+1 < len(p.rs) {
					p.i += 2
					continue
				}
				if p.rs[p.i] == ')' {
					url := string(p.rs[urlStart:p.i])
					p.i++
					text = linkPart(text, ']', "link text", p.sanitize)
					url = linkPart(url, ')', "link url", p.sanitize)
					if text == "" || url == "" {
						p.i = start
						return "", false
					}
					return "[" + text + "](" + url + ")", true
				}
				p.i++
			}
			p.i = start
			return "", false
		}
		p.i++
	}
	p.i = start
	return "", false
}

// A link's two halves are checked the same way: walk the runes honouring
// backslash escapes and reject (or escape) the one delimiter that would end the
// half early - ] inside the text, ) inside the URL.

// linkPart repairs one half of a link: valid input passes through, invalid
// input is escaped when sanitizing and dropped when validating.
func linkPart(s string, delim rune, what string, sanitize bool) string {
	if err := validateUnescaped(s, delim, what); err == nil {
		return s
	}
	if !sanitize {
		return ""
	}
	return escapeUnescaped(s, delim)
}

// validateUnescaped reports an unescaped delim, or a trailing backslash that
// escapes nothing.
func validateUnescaped(s string, delim rune, what string) error {
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if rs[i] == '\\' {
			if i+1 >= len(rs) {
				return errInvalidMarkdown
			}
			i += 2
			continue
		}
		if rs[i] == delim {
			return fmt.Errorf("%w: unescaped %c in %s", errInvalidMarkdown, delim, what)
		}
		i++
	}
	return nil
}

// escapeUnescaped backslash-escapes every unescaped delim, leaving existing
// escape pairs alone.
func escapeUnescaped(s string, delim rune) string {
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if rs[i] == '\\' && i+1 < len(rs) {
			b.WriteRune(rs[i])
			b.WriteRune(rs[i+1])
			i += 2
			continue
		}
		if rs[i] == delim {
			b.WriteRune('\\')
		}
		b.WriteRune(rs[i])
		i++
	}
	return b.String()
}
