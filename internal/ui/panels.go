package ui

// dashboardPanel identifies a panel in Tab navigation order.
type dashboardPanel uint8

const (
	panelPrinters dashboardPanel = iota
	panelDetails
	panelNotices
	panelCount
)
