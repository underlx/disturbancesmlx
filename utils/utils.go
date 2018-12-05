package utils

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"sort"
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

	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		return ip
	}
	return host
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

// FormatPortugueseMonthShort returns the Portuguese name abbreviation for a month
func FormatPortugueseMonthShort(month time.Month) string {
	switch month {
	case time.January:
		return "Jan"
	case time.February:
		return "Fev"
	case time.March:
		return "Mar"
	case time.April:
		return "Abr"
	case time.May:
		return "Mai"
	case time.June:
		return "Jun"
	case time.July:
		return "Jul"
	case time.August:
		return "Ago"
	case time.September:
		return "Set"
	case time.October:
		return "Out"
	case time.November:
		return "Nov"
	case time.December:
		return "Dez"
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

// StationConnectionURLs returns a mapping from locales to connection types to URLs
// of the HTML files containing the connection info for the given station
func StationConnectionURLs(station *dataobjects.Station) map[string]map[string]string {
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

// SchedulesToLines converts a lobby schedule to a set of human-readable lines
func SchedulesToLines(schedules []*dataobjects.LobbySchedule) []string {
	schedulesByDay := make(map[int]*dataobjects.LobbySchedule)
	exceptions := []struct {
		day      int
		schedule *dataobjects.LobbySchedule
	}{}
	for _, schedule := range schedules {
		if schedule.Holiday && schedule.Day == 0 {
			schedulesByDay[-1] = schedule
		} else if !schedule.Holiday {
			schedulesByDay[schedule.Day] = schedule
		} else {
			exceptions = append(exceptions, struct {
				day      int
				schedule *dataobjects.LobbySchedule
			}{schedule.Day, schedule})
		}
	}

	weekdaysAllTheSame := true
	for i := 2; i < 6; i++ {
		if !schedulesByDay[1].Compare(schedulesByDay[i]) {
			weekdaysAllTheSame = false
		}
	}

	holidaysAllTheSame := schedulesByDay[-1].Compare(schedulesByDay[0]) && schedulesByDay[6].Compare(schedulesByDay[0])
	allDaysTheSame := weekdaysAllTheSame && holidaysAllTheSame && schedulesByDay[-1].Compare(schedulesByDay[2])

	scheduleString := []string{}

	if allDaysTheSame {
		scheduleString = []string{"Todos os dias: " + scheduleToString(schedulesByDay[0])}
	} else if weekdaysAllTheSame {
		scheduleString = append(scheduleString, "Dias úteis: "+scheduleToString(schedulesByDay[1]))
	} else {
		for i := 2; i < 6; i++ {
			scheduleString = append(scheduleString, time.Weekday(i).String()+": "+scheduleToString(schedulesByDay[i]))
		}
	}

	if !allDaysTheSame && holidaysAllTheSame {
		scheduleString = append(scheduleString, "Fins de semana e feriados: "+scheduleToString(schedulesByDay[0]))
	} else if !allDaysTheSame {
		scheduleString = append(scheduleString, time.Weekday(0).String()+": "+scheduleToString(schedulesByDay[0]))
		scheduleString = append(scheduleString, time.Weekday(6).String()+": "+scheduleToString(schedulesByDay[6]))
		scheduleString = append(scheduleString, "Feriados: "+scheduleToString(schedulesByDay[-1]))
	}

	if len(exceptions) == 0 {
		return scheduleString
	}

	sort.Slice(exceptions, func(i, j int) bool { return exceptions[i].day < exceptions[j].day })

	location, err := time.LoadLocation(exceptions[0].schedule.Lobby.Station.Network.Timezone)
	if err != nil {
		return scheduleString
	}

	now := time.Now().In(location)
	currentDay := now.YearDay()

	for _, exception := range exceptions {
		if exception.day >= currentDay-1 { // minus one, because schedules from previous days may extend past midnight
			// time.Date takes care of overflowing day into month
			date := time.Date(now.Year(), 1, exception.day, 0, 0, 0, 0, location)
			scheduleString = append(scheduleString,
				fmt.Sprintf("%d de %s: %s",
					date.Day(),
					FormatPortugueseMonth(date.Month()),
					scheduleToString(exception.schedule)))
		}
	}

	return scheduleString
}
func scheduleToString(schedule *dataobjects.LobbySchedule) string {
	if !schedule.Open {
		return "encerrado"
	}
	openString := time.Time(schedule.OpenTime).Format("15:04")
	closeString := time.Time(schedule.OpenTime).
		Add(time.Duration(schedule.OpenDuration)).Format("15:04")
	text := fmt.Sprintf("%s - %s", openString, closeString)
	hours := time.Duration(schedule.OpenDuration).Hours()
	if hours >= 24 {
		text += fmt.Sprintf(" (%s horas)",
			strings.TrimSuffix(fmt.Sprintf("%.2f", hours), ".00"))
	}
	return text
}
