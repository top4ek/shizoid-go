package version

// commit is set at link time via -ldflags "-X apps/shizoid/internal/version.commit=...".
var commit = "unknown"

func Version() string {
	return commit
}
