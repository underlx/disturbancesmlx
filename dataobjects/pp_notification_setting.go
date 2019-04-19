package dataobjects

import (
	"errors"
	"fmt"

	"github.com/gbl08ma/sqalx"
	sq "github.com/gbl08ma/squirrel"
)

// PPNotificationSetting is a PosPlay notification setting
type PPNotificationSetting struct {
	DiscordID        uint64
	NotificationType string
	Method           string
	Enabled          bool
}

func getPPNotificationSettingsWithSelect(node sqalx.Node, sbuilder sq.SelectBuilder) ([]*PPNotificationSetting, error) {
	settings := []*PPNotificationSetting{}

	tx, err := node.Beginx()
	if err != nil {
		return settings, err
	}
	defer tx.Commit() // read-only tx

	rows, err := sbuilder.Columns("pp_notification_setting.discord_id", "pp_notification_setting.notification_type",
		"pp_notification_setting.method", "pp_notification_setting.enabled").
		From("pp_notification_setting").
		RunWith(tx).Query()
	if err != nil {
		return settings, fmt.Errorf("getPPNotificationSettingsWithSelect: %s", err)
	}

	for rows.Next() {
		var setting PPNotificationSetting
		err := rows.Scan(
			&setting.DiscordID,
			&setting.NotificationType,
			&setting.Method,
			&setting.Enabled)
		if err != nil {
			rows.Close()
			return settings, fmt.Errorf("getPPNotificationSettingsWithSelect: %s", err)
		}
		settings = append(settings, &setting)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return settings, fmt.Errorf("getPPNotificationSettingsWithSelect: %s", err)
	}
	rows.Close()
	return settings, nil
}

// GetPPNotificationSetting returns the setting for the given parameters
func GetPPNotificationSetting(node sqalx.Node, discordID uint64, notifType string, method string, defaultValues map[string]map[string]bool) (bool, error) {
	s := sdb.Select().
		Where(sq.Eq{"discord_id": discordID}).
		Where(sq.Eq{"notification_type": notifType}).
		Where(sq.Eq{"method": method})

	defaultValue := defaultValues[notifType][method]

	settings, err := getPPNotificationSettingsWithSelect(node, s)
	if err != nil {
		return defaultValue, err
	}
	if len(settings) == 0 {
		return defaultValue, nil
	}
	return settings[0].Enabled, nil
}

// SetPPNotificationSetting sets the setting for the given parameters
func SetPPNotificationSetting(node sqalx.Node, discordID uint64, notifType string, method string, enabled bool) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	s := sdb.Select().
		Where(sq.Eq{"discord_id": discordID}).
		Where(sq.Eq{"notification_type": notifType}).
		Where(sq.Eq{"method": method})

	settings, err := getPPNotificationSettingsWithSelect(tx, s)
	if err != nil {
		return err
	}
	if len(settings) == 0 {
		settings = []*PPNotificationSetting{
			&PPNotificationSetting{
				DiscordID:        discordID,
				NotificationType: notifType,
				Method:           method,
			},
		}
	}
	settings[0].Enabled = enabled
	err = settings[0].Update(tx)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Update adds or updates the PPNotificationSetting
func (setting *PPNotificationSetting) Update(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Insert("pp_notification_setting").
		Columns("discord_id", "notification_type", "method", "enabled").
		Values(setting.DiscordID, setting.NotificationType, setting.Method, setting.Enabled).
		Suffix("ON CONFLICT (discord_id, notification_type, method) DO UPDATE SET enabled = ?",
			setting.Enabled).
		RunWith(tx).Exec()

	if err != nil {
		return errors.New("AddPPNotificationSetting: " + err.Error())
	}
	return tx.Commit()
}

// Delete deletes the PPNotificationSetting
func (setting *PPNotificationSetting) Delete(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = sdb.Delete("pp_notification_setting").
		Where(sq.Eq{"discord_id": setting.DiscordID}).
		Where(sq.Eq{"notification_type": setting.NotificationType}).
		Where(sq.Eq{"method": setting.Method}).
		RunWith(tx).Exec()
	if err != nil {
		return fmt.Errorf("RemovePPNotificationSetting: %s", err)
	}
	return tx.Commit()
}
