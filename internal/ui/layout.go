package ui

// tableColumns describes widths shared by the header and every printer row.
type tableColumns struct {
	name, link, state, progress, remaining, finish, notices int
}

func columnsFor(width int) tableColumns {
	if width < compactTableWidth {
		return tableColumns{name: 21, state: 12, progress: 12}
	}
	if width < fullTableWidth {
		return tableColumns{name: 22, link: 10, state: 12, progress: 9, remaining: 11}
	}
	return tableColumns{name: 23, link: 10, state: 12, progress: 16, remaining: 11, finish: 10, notices: 4}
}
