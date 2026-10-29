package buildinfo

import "fmt"

// Release builds inject these through -ldflags. Direct go builds remain useful.
var (
	Version = "0.4.6"
	Commit  = "unknown"
	Date    = "unknown"
)

func String() string { return fmt.Sprintf("bambu-tui %s (commit %s, built %s)", Version, Commit, Date) }
