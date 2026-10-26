package main

import "time"

// CLI identifiers and local input/queue policies.
const (
	commandMonitor              = "monitor"
	commandLogin                = "login"
	commandLogout               = "logout"
	commandDoctor               = "doctor"
	notificationShutdownTimeout = 5 * time.Second
	eventBufferSize             = 128
	promptWidth                 = 48
	promptCharacterLimit        = 16384
	keyQuit                     = "ctrl+c"
	keyEscape                   = "esc"
	keyEnter                    = "enter"
)
