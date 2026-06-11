package captcha

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"

	"shizoid/internal/locale"
)

// buildChallenge picks one correct symbol and three decoys, shuffled into four buttons.
func buildChallenge(lang string) (correct locale.Symbol, buttons []locale.Symbol, err error) {
	symbols := locale.Symbols(lang, "captcha.symbols")
	if len(symbols) < 4 {
		return locale.Symbol{}, nil, fmt.Errorf("captcha: need at least 4 symbols, got %d", len(symbols))
	}
	correctIdx := rand.IntN(len(symbols))
	correct = symbols[correctIdx]

	pool := make([]locale.Symbol, 0, len(symbols)-1)
	for i, s := range symbols {
		if i != correctIdx {
			pool = append(pool, s)
		}
	}
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	buttons = append(pool[:3], correct)
	rand.Shuffle(len(buttons), func(i, j int) { buttons[i], buttons[j] = buttons[j], buttons[i] })
	return correct, buttons, nil
}

func callbackData(userID int64, emoji string) string {
	return CallbackPrefix + strconv.FormatInt(userID, 10) + ":" + emoji
}

func parseCallback(data string) (userID int64, emoji string, ok bool) {
	rest := strings.TrimPrefix(data, CallbackPrefix)
	userPart, emoji, found := strings.Cut(rest, ":")
	if !found || userPart == "" || emoji == "" {
		return 0, "", false
	}
	id, err := strconv.ParseInt(userPart, 10, 64)
	if err != nil {
		return 0, "", false
	}
	return id, emoji, true
}
