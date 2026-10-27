package notifications

// MessageKind describes the meaning of Last, independently of its wording,
// activity retention, or the UI's choice of color and display lifetime.
type MessageKind uint8

const (
	MessageInfo MessageKind = iota
	MessageSuccess
	MessageWarning
	MessageError
)
