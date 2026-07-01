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

// FormatTemplate escapes template literals while preserving %{placeholder} tokens.
func FormatTemplate(s string) string {
	var b strings.Builder
	for {
		idx := strings.Index(s, "%{")
		if idx < 0 {
			b.WriteString(bot.EscapeMarkdownUnescaped(s))
			return b.String()
		}
		if idx > 0 {
			b.WriteString(bot.EscapeMarkdownUnescaped(s[:idx]))
		}
		end := strings.Index(s[idx:], "}")
		if end < 0 {
			b.WriteString(bot.EscapeMarkdownUnescaped(s[idx:]))
			return b.String()
		}
		end += idx
		b.WriteString(s[idx : end+1])
		s = s[end+1:]
	}
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
					text = sanitizeLinkPart(text, validateLinkText, sanitizeLinkText, p.sanitize)
					url = sanitizeLinkPart(url, validateLinkURL, sanitizeLinkURL, p.sanitize)
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

func sanitizeLinkPart(s string, validate func(string) error, sanitize func(string) string, doSanitize bool) string {
	if err := validate(s); err != nil {
		if !doSanitize {
			return ""
		}
		return sanitize(s)
	}
	return s
}

func validateLinkText(s string) error {
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if rs[i] == '\\' {
			if i+1 >= len(rs) {
				return errInvalidMarkdown
			}
			i += 2
			continue
		}
		if rs[i] == ']' {
			return fmt.Errorf("%w: unescaped ] in link text", errInvalidMarkdown)
		}
		i++
	}
	return nil
}

func validateLinkURL(s string) error {
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if rs[i] == '\\' {
			if i+1 >= len(rs) {
				return errInvalidMarkdown
			}
			i += 2
			continue
		}
		if rs[i] == ')' {
			return fmt.Errorf("%w: unescaped ) in link url", errInvalidMarkdown)
		}
		i++
	}
	return nil
}

func sanitizeLinkText(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if rs[i] == '\\' && i+1 < len(rs) {
			b.WriteRune(rs[i])
			b.WriteRune(rs[i+1])
			i += 2
			continue
		}
		if rs[i] == ']' {
			b.WriteString(`\]`)
			i++
			continue
		}
		b.WriteRune(rs[i])
		i++
	}
	return b.String()
}

func sanitizeLinkURL(s string) string {
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if rs[i] == '\\' && i+1 < len(rs) {
			b.WriteRune(rs[i])
			b.WriteRune(rs[i+1])
			i += 2
			continue
		}
		if rs[i] == ')' {
			b.WriteString(`\)`)
			i++
			continue
		}
		b.WriteRune(rs[i])
		i++
	}
	return b.String()
}
