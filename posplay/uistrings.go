package posplay

import (
	"fmt"

	"github.com/underlx/disturbancesmlx/dataobjects"
)

// DescriptionForXPTransaction returns a human-friendly description of a XP transaction
func DescriptionForXPTransaction(tx *dataobjects.PPXPTransaction) string {
	extra := tx.UnmarshalExtra()
	switch tx.Type {
	case "SIGNUP_BONUS":
		return "Oferta de boas-vindas"
	case "PAIR_BONUS":
		return "Associação de dispositivo"
	case "TRIP_SUBMIT_REWARD":
		numstations, ok := extra["station_count"].(float64)
		numexchanges, ok2 := extra["interchange_count"].(float64)
		offpeak, ok3 := extra["offpeak"].(bool)
		if ok && ok2 && ok3 {
			excstr := ""
			switch int(numexchanges) {
			case 0:
				excstr = ""
			case 1:
				excstr = ", com 1 troca de linha"
			default:
				excstr = fmt.Sprintf(", com %d trocas de linha", int(numexchanges))
			}
			ofpstr := ""
			if offpeak {
				ofpstr = ", fora das horas de ponta"
			}
			return fmt.Sprintf("Viagem por %d estações%s%s", int(numstations), excstr, ofpstr)
		}
		return "Viagem"
	case "TRIP_CONFIRM_REWARD":
		return "Verificação de registo de viagem"
	case "DISCORD_REACTION_EVENT":
		return "Participação em evento no Discord do UnderLX"
	case "DISCORD_CHALLENGE_EVENT":
		return "Participação em desafio no Discord do UnderLX"
	case "DISCORD_PARTICIPATION":
		return "Participação na discussão no Discord do UnderLX"
	case "ACHIEVEMENT_REWARD":
		id, ok := extra["achievement_id"].(string)
		if ok {
			allAchievementsMutex.RLock()
			defer allAchievementsMutex.RUnlock()
			a, ok2 := allAchievementsByID[id]
			if ok2 {
				return fmt.Sprintf("Proeza \"%s\" alcançada", a.Names[a.MainLocale])
			}
		}
		return "Proeza alcançada"
	default:
		// ideally this should never show
		return "Bónus genérico"
	}
}

// DescriptionForRarity returns a human-friendly description of a achievement rarity value (range 0 to 100)
func DescriptionForRarity(rarity float64) string {
	switch {
	case rarity <= 1:
		return "extremamente rara"
	case rarity <= 8:
		return "rara"
	case rarity <= 25:
		return "incomum"
	case rarity <= 50:
		return "comum"
	case rarity <= 75:
		return "muito comum"
	case rarity < 100:
		return "extremamente comum"
	case rarity >= 100:
		return "universal"
	}
	return ""
}

// NameForNotificationType returns a human-friendly name of a notification type
func NameForNotificationType(notifType string) string {
	switch notifType {
	case NotificationTypeAchievementAchieved:
		return "Proeza alcançada"
	case NotificationTypeGuildEventWon:
		return "Participação em evento no Discord do UnderLX"
	}
	return ""
}

// NameForNotificationMethod returns a human-friendly name of a notification method
func NameForNotificationMethod(notifMethod string) string {
	switch notifMethod {
	case NotificationMethodDiscordDM:
		return "Mensagem directa pelo Discord"
	case NotificationMethodAppNotif:
		return "Notificação na aplicação"
	}
	return ""
}
