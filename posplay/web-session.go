package posplay

import "net/http"

func forceLogin(w http.ResponseWriter, r *http.Request) {
	_, redirected, err := GetSession(r, w, true)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if !redirected {
		http.Redirect(w, r, BaseURL(), http.StatusTemporaryRedirect)
	}
}

func forceLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	session, _, err := GetSession(r, w, false)
	if err != nil {
		config.Log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if session != nil {
		session.Logout(r, w)
	}
	http.Redirect(w, r, BaseURL(), http.StatusTemporaryRedirect)
}
