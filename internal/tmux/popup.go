package tmux

// DashboardPopup shows the taux dashboard in a tmux popup.
func DashboardPopup() error {
	return DisplayPopup(PopupOpts{
		Width:   "80%",
		Height:  "80%",
		Title:   " taux ",
		Command: "taux dashboard",
	})
}

// ActiveListPopup shows active sessions popup.
func ActiveListPopup() error {
	return DisplayPopup(PopupOpts{
		Width:   "60%",
		Height:  "50%",
		Title:   " Active Sessions ",
		Command: "taux get sessions -s active",
	})
}

// StatsPopup shows stats popup.
func StatsPopup() error {
	return DisplayPopup(PopupOpts{
		Width:   "50%",
		Height:  "40%",
		Title:   " Stats ",
		Command: "taux get stats",
	})
}
