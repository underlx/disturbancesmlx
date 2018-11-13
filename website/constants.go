// +build !release

package website

const (
	// DEBUG is whether this is a debug build
	DEBUG = true

	// MLnetworkID is the ID of the main network
	MLnetworkID = "pt-ml"

	// CSRFfieldName is the name of the form field used for CSRF protection
	CSRFfieldName = "disturbances.csrf"

	// CSRFcookieName is the name of the cookie used for CSRF protection
	CSRFcookieName = "_disturbances_csrf"
)
