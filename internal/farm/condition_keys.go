package farm

// Condition key components are persisted identities; their spelling is stable.
const (
	ConditionCloud          = "cloud"
	ConditionOffline        = "offline"
	ConditionStale          = "stale"
	ConditionState          = "state"
	ConditionFaultPrefix    = "fault/"
	ConditionLowPrefix      = "low/"
	lowFilamentKeyFormat    = "%s/low/%d/%d"
	lowFilamentFamilyFormat = "low/%d/%d"
)
