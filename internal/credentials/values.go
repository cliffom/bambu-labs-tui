package credentials

import "time"

// Host credential command limits and exit codes; command argv remains explicit.
const (
	nativeCommandTimeout   = 15 * time.Second
	keychainNotFoundExit   = 44
	secretToolNotFoundExit = 1
	keychainMaxInputBytes  = 4095
	probeRandomBytes       = 12
	servicePrefix          = "com.github.cliffom.bambu-tui."
)
