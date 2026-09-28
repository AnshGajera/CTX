package version

import "fmt"

// Info holds build-time version metadata.
type Info struct {
	Version string
	Commit  string
	Date    string
}

var (
	Version = "0.1.5"
	Commit  = "none"
	Date    = "unknown"
)

// Current returns the build-time Info.
func Current() Info {
	return Info{Version: Version, Commit: Commit, Date: Date}
}

// String returns a human-readable version string.
func (i Info) String() string {
	return fmt.Sprintf("ctx %s (commit %s, built %s)", i.Version, i.Commit, i.Date)
}
