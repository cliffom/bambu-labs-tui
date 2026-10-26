package notificationstate

// These reasons describe existing delivery policy, not physical fault resolution.
const (
	SuppressedReviewed      = "reviewed"
	SuppressedNotActionable = "not_actionable"
	SuppressedAccepted      = "already_accepted"
	SuppressedFailed        = "already_failed"
	SuppressedReserved      = "already_reserved"
	SuppressedBaseline      = "completion_baseline"
	SuppressedJobIdentity   = "missing_job_identity"
)

func attemptReason(attempt Attempt) string {
	switch attempt {
	case Accepted:
		return SuppressedAccepted
	case Failed:
		return SuppressedFailed
	default:
		return SuppressedReserved
	}
}
