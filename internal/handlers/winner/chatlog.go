package winner

import (
	"strings"
	"time"

	"shizoid/internal/config"
	"shizoid/internal/models"
	"shizoid/internal/utils"
)

const (
	// window is how far back the announcement looks for chat news.
	window = 24 * time.Hour
	// budgetMargin mirrors the scheduler's summary margin: slack for the role
	// labels and JSON envelope the byte budget cannot account for.
	budgetMargin = 256

	logHeader = "Chat log:\n"

	// drawReserve is the slice of the context budget the draw block gets: the
	// prize and one line per scorer, each carrying that scorer's own words.
	drawReserve = 2048
	// scorerQuotes and scorerQuoteRunes bound one scorer's words of the day, so
	// a single talkative member cannot eat the whole draw block.
	scorerQuotes     = 2
	scorerQuoteRunes = 120
)

// logBudget is what is left of the summary context once the system prompt, the
// draw block and the log header are paid for.
func logBudget(systemBytes int) int {
	return config.MaxSummaryContextBytes - systemBytes - drawReserve - len(logHeader) - budgetMargin
}

// scorerExcerpt returns the newest few things a scorer said, as material for the
// model to retell. It is not quoted verbatim in the announcement: the prompt
// forbids that.
func scorerExcerpt(rows []models.MessageRow, userID int64) string {
	var out []string
	for _, row := range rows {
		if row.UserID != userID {
			continue
		}
		text := oneLine(row.Text)
		if text == "" {
			continue
		}
		out = append(out, truncateRunes(text, scorerQuoteRunes))
		if len(out) == scorerQuotes {
			break
		}
	}
	return strings.Join(out, " / ")
}

func truncateRunes(s string, max int) string {
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max])
}

// formatLog turns newest-first rows into chronological "Name: text" lines,
// dropping the bot's own messages so an announcement never reports on itself.
// One row must yield exactly one line: the whole log is flattened into a single
// user message, so a member whose text contained a newline could otherwise forge
// an extra "Name: text" entry.
func formatLog(rows []models.MessageRow, botID int64) []string {
	lines := make([]string, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		text := strings.TrimSpace(row.Text)
		if text == "" {
			continue
		}
		if row.UserID == botID || (row.IsBot.Valid && row.IsBot.Bool) {
			continue
		}
		name := utils.DisplayName(row.Username.String, row.FirstName.String, row.LastName.String)
		lines = append(lines, oneLine(name)+": "+oneLine(text))
	}
	return lines
}

// oneLine collapses the line breaks that would let a message forge a log entry.
func oneLine(s string) string {
	return strings.TrimSpace(lineBreaks.Replace(s))
}

var lineBreaks = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

// fitLog drops the oldest lines until the rendered log fits the budget. The
// query budget counts message text only, while the log also carries a name per
// line, so the excess is trimmed here.
func fitLog(lines []string, budget int) []string {
	size := 0
	for _, l := range lines {
		size += len(l) + 1
	}
	for size > budget && len(lines) > 0 {
		size -= len(lines[0]) + 1
		lines = lines[1:]
	}
	return lines
}
