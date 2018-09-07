package utils

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/underlx/disturbancesmlx/dataobjects"
)

// SupportedLocales contains the supported locales for extra and meta content
var SupportedLocales = [...]string{"pt", "en", "es", "fr"}

// RequestIsTLS returns whether a request was made over a HTTPS channel
// Looks at the appropriate headers if the server is behind a proxy
func RequestIsTLS(r *http.Request) bool {
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.Header.Get("X-Forwarded-Proto") == "HTTPS" {
		return true
	}
	return r.TLS != nil
}

// GetClientIP retrieves the client IP address from the request information.
// It detects common proxy headers to return the actual client's IP and not the proxy's.
func GetClientIP(r *http.Request) (ip string) {
	var pIPs string
	var pIPList []string

	if pIPs = r.Header.Get("X-Real-Ip"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("Real-Ip"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("X-Forwarded-For"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("X-Forwarded"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("Forwarded-For"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else if pIPs = r.Header.Get("Forwarded"); pIPs != "" {
		pIPList = strings.Split(pIPs, ",")
		ip = strings.TrimSpace(pIPList[0])

	} else {
		ip = r.RemoteAddr
	}

	return strings.Split(ip, ":")[0]
}

// FormatPortugueseMonth returns the Portuguese name for a month
func FormatPortugueseMonth(month time.Month) string {
	switch month {
	case time.January:
		return "Janeiro"
	case time.February:
		return "Fevereiro"
	case time.March:
		return "Março"
	case time.April:
		return "Abril"
	case time.May:
		return "Maio"
	case time.June:
		return "Junho"
	case time.July:
		return "Julho"
	case time.August:
		return "Agosto"
	case time.September:
		return "Setembro"
	case time.October:
		return "Outubro"
	case time.November:
		return "Novembro"
	case time.December:
		return "Dezembro"
	default:
		return ""
	}
}

// ComputeStationTriviaURLs returns a mapping from locales to URLs of the HTML file containing the trivia for the given station
func ComputeStationTriviaURLs(station *dataobjects.Station) map[string]string {
	m := make(map[string]string)
	for _, locale := range SupportedLocales {
		m[locale] = "stationkb/" + locale + "/trivia/" + station.ID + ".html"
	}
	return m
}

// ComputeStationConnectionURLs returns a mapping from locales to connection types to URLs
// of the HTML files containing the connection info for the given station
func ComputeStationConnectionURLs(station *dataobjects.Station) map[string]map[string]string {
	m := make(map[string]map[string]string)
	connections := []string{"boat", "bus", "train", "park", "bike"}
	for _, locale := range SupportedLocales {
		for _, connection := range connections {
			path := "stationkb/" + locale + "/connections/" + connection + "/" + station.ID + ".html"
			if info, err := os.Stat(path); err == nil && !info.IsDir() {
				if m[connection] == nil {
					m[connection] = make(map[string]string)
				}
				m[connection][locale] = path
			}
		}
	}
	return m
}
