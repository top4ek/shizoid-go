package neural

type SamplingParams struct {
	Temperature       float64 `yaml:"temperature"`
	TopP              float64 `yaml:"top_p"`
	TopK              int     `yaml:"top_k"`
	MinP              float64 `yaml:"min_p"`
	PresencePenalty   float64 `yaml:"presence_penalty"`
	RepetitionPenalty float64 `yaml:"repetition_penalty"`
}

// Provider describes one OpenAI-compatible endpoint in a fallback chain. The
// internal llama.cpp server is just another entry (it speaks the same API).
type Provider struct {
	Name           string `yaml:"name"`
	BaseURL        string `yaml:"base_url"`
	APIKey         string `yaml:"api_key"`
	Model          string `yaml:"model"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
	// SlotCheck probes GET /health?fail_on_no_slot=1 before chat/completions.
	// Enable for llama.cpp so a full slot pool skips immediately instead of
	// waiting in the server's deferred queue until the client timeout fires.
	SlotCheck bool `yaml:"slot_check"`
	// DailyLimit caps successful completions per calendar day (shared by name
	// across reply and summary chains). Zero or omitted means unlimited.
	DailyLimit int `yaml:"daily_limit"`
	// ContextSize is the max UTF-8 byte budget for the full chat completion
	// payload (system + history). Zero means no trim for this provider.
	ContextSize int `yaml:"context_size"`
	// Sampling holds optional generation parameters for this provider.
	Sampling *SamplingParams `yaml:"sampling"`
}
