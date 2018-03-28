package main

import (
	"net/http"
	"time"

	uuid "github.com/satori/go.uuid"
)

// Session represents a user session
type Session struct {
	UserID      string
	DisplayName string
	IsAdmin     bool
}

// AuthHandler serves requests from users that come from the SSO login page
func AuthHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("from_sso_server") != "1" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	session, _ := sessionStore.Get(r, "internal")
	if session.IsNew || session.Values["id"] == nil || session.Values["rid"] == nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ssoID := r.URL.Query().Get("sso_id")
	ssoID2 := r.URL.Query().Get("sso_id2")
	rid := session.Values["rid"].(string)
	login, err := daClient.GetLogin(ssoID, 7*24*60*60, nil, ssoID2, rid)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if login.RecoveredInfo != session.Values["id"].(string) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	session.Values["admin"] = login.Admin || login.TagMap["sso_admin"] || login.TagMap["underlx_admin"]

	session.Values["ssoid"] = login.SSOID
	session.Values["userid"] = login.UserID
	session.Values["displayname"] = login.FieldMap["displayname"]
	session.Values["authenticated"] = time.Now().UTC().Unix()
	err = session.Save(r, w)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, session.Values["original_url"].(string), http.StatusFound)
}

// AuthLogoutHandler serves requests from users that come from the SSO login page
func AuthLogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := sessionStore.Get(r, "internal")
	if !session.IsNew && session.Values["ssoid"] != nil {
		daClient.Logout(session.Values["ssoid"].(string))
	}
	session.Values["id"] = nil
	session.Values["authenticated"] = int64(0)
	err := session.Save(r, w)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, websiteURL, http.StatusFound)
}

// AuthGetSession retrieves the Session associated with the user of the specified request, if one exists
func AuthGetSession(w http.ResponseWriter, r *http.Request, doLogin bool) (bool, Session, error) {
	session, _ := sessionStore.Get(r, "internal")
	if session.IsNew || session.Values["authenticated"] == nil || session.Values["authenticated"].(int64) < time.Now().UTC().AddDate(0, 0, -7).Unix() {
		if !doLogin {
			return false, Session{}, nil
		}
		id, err := uuid.NewV4()
		if err != nil {
			return false, Session{}, err
		}
		session.Values["id"] = id.String()
		session.Values["original_url"] = r.URL.String()

		url, rid, err := daClient.InitLogin(websiteURL+"/auth", false, "", nil, session.Values["id"].(string), websiteURL)

		if err != nil {
			return false, Session{}, err
		}

		session.Values["rid"] = rid
		err = session.Save(r, w)
		if err != nil {
			return false, Session{}, err
		}

		http.Redirect(w, r, url, http.StatusFound)
		return false, Session{}, nil
	}

	return true, Session{
		UserID:      session.Values["userid"].(string),
		DisplayName: session.Values["displayname"].(string),
		IsAdmin:     session.Values["admin"].(bool),
	}, nil
}
