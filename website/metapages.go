package website

import (
	"net/http"

	"github.com/gorilla/mux"
)

// meta-pages have been moved to underlx.com, now these are just redirects

// AboutPage serves the about page
func AboutPage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://underlx.com/about", http.StatusMovedPermanently)
}

// DonatePage serves the donations page
func DonatePage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "https://underlx.com/donate", http.StatusMovedPermanently)
}

// PrivacyPolicyPage serves the privacy policy page
func PrivacyPolicyPage(w http.ResponseWriter, r *http.Request) {
	if mux.Vars(r)["lang"] != "en" {
		http.Redirect(w, r, "https://underlx.com/privacy", http.StatusMovedPermanently)
	} else {
		http.Redirect(w, r, "https://underlx.com/privacy/en", http.StatusMovedPermanently)
	}
}

// TermsPage serves the terms and conditions page
func TermsPage(w http.ResponseWriter, r *http.Request) {
	if mux.Vars(r)["lang"] != "en" {
		http.Redirect(w, r, "https://underlx.com/terms", http.StatusMovedPermanently)
	} else {
		http.Redirect(w, r, "https://underlx.com/terms/en", http.StatusMovedPermanently)
	}
}
