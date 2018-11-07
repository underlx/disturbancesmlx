package website

import (
	"net/http"

	"github.com/gorilla/mux"
)

// AboutPage serves the about page
func AboutPage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	p := struct {
		PageCommons
	}{}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Sobre nós")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "about.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// DonatePage serves the donations page
func DonatePage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	p := struct {
		PageCommons
	}{}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Donativos")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = webtemplate.ExecuteTemplate(w, "donate.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// PrivacyPolicyPage serves the privacy policy page
func PrivacyPolicyPage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	p := struct {
		PageCommons
	}{}

	if mux.Vars(r)["lang"] != "en" {
		p.PageCommons, err = InitPageCommons(tx, w, r, "Política de privacidade")
	} else {
		p.PageCommons, err = InitPageCommons(tx, w, r, "Privacy Policy")
	}
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if mux.Vars(r)["lang"] != "en" {
		err = webtemplate.ExecuteTemplate(w, "privacy.html", p)
	} else {
		err = webtemplate.ExecuteTemplate(w, "privacy-en.html", p)
	}
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// TermsPage serves the terms and conditions page
func TermsPage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	p := struct {
		PageCommons
	}{}

	if mux.Vars(r)["lang"] != "en" {
		p.PageCommons, err = InitPageCommons(tx, w, r, "Termos e Condições")
	} else {
		p.PageCommons, err = InitPageCommons(tx, w, r, "Terms")
	}
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if mux.Vars(r)["lang"] != "en" {
		err = webtemplate.ExecuteTemplate(w, "terms.html", p)
	} else {
		err = webtemplate.ExecuteTemplate(w, "terms-en.html", p)
	}
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
