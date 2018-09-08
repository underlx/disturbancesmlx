// +build !release

package posplay

const (
	// DEBUG is whether this is a debug build
	DEBUG = true

	// SessionName is the name of the PosPlay session in the session store
	SessionName = "posplay"

	// GameTimezone is the timezone where the game is played
	GameTimezone = "Europe/Lisbon"

	// CSRFfieldName is the name of the form field used for CSRF protection
	CSRFfieldName = "posplay.csrf"
)
