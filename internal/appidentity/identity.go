// Package appidentity defines the application namespace and legacy migration support.
package appidentity

const (
	Namespace = "com.suitepotato.bambustand"
	// LegacyDirectory and LegacyService are lookup-only upgrade identifiers.
	LegacyDirectory = "bambu-tui"
	LegacyService   = "com.github.cliffom.bambu-tui."
)
