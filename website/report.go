package website

import (
	"net/http"

	"github.com/underlx/disturbancesmlx/dataobjects"
	"github.com/underlx/disturbancesmlx/utils"
)

// ReportPage serves the disturbance reporting page
func ReportPage(w http.ResponseWriter, r *http.Request) {
	tx, err := rootSqalxNode.Beginx()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		webLog.Println(err)
		return
	}
	defer tx.Commit()

	p := struct {
		PageCommons
		Message         string
		MessageIsError  bool
		ReportableLines []*dataobjects.Line
	}{}

	p.PageCommons, err = InitPageCommons(tx, w, r, "Comunicar problemas na circulação")
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, line := range p.Lines {
		if closed, err := line.CurrentlyClosed(tx); err == nil && !closed {
			p.ReportableLines = append(p.ReportableLines, line.Line)
		}
	}

	if r.Method == http.MethodPost {
		err := r.ParseForm()
		if err != nil {
			webLog.Println(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if !webcaptcha.Verify(*r) {
			p.Message = "A verificação do reCAPTCHA falhou."
			p.MessageIsError = true
		} else {
			oneSucceeded := false
			for _, value := range r.Form["lines"] {
				line, err := dataobjects.GetLine(tx, value)
				if err != nil {
					webLog.Println(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				report := dataobjects.NewLineDisturbanceReport(utils.GetClientIP(r), line, "general")

				err = reportHandler.HandleLineDisturbanceReport(report)
				if err == nil {
					oneSucceeded = true
				}
			}

			if len(r.Form["lines"]) == 0 {
				p.Message = "Seleccione as linhas em que verifica problemas. Se não verifica problemas em nenhuma linha, não comunique nada. Agradecemos a sua participação."
				p.MessageIsError = true
			} else if oneSucceeded {
				p.Message = "Relato registado. Agradecemos a sua participação."
				p.MessageIsError = false
			} else {
				p.Message = "O seu relato para este problema já tinha sido registado recentemente. Agradecemos a sua participação."
				p.MessageIsError = true
			}
		}
	}

	err = webtemplate.ExecuteTemplate(w, "report.html", p)
	if err != nil {
		webLog.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
