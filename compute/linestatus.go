package compute

import (
	"github.com/gbl08ma/sqalx"
	"github.com/underlx/disturbancesmlx/dataobjects"
)

// UpdateStatusMsgTypes updates the MsgTypes of existing line statuses
func UpdateStatusMsgTypes(node sqalx.Node) error {
	tx, err := node.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	statuses, err := dataobjects.GetStatuses(tx)
	if err != nil {
		return err
	}
	for _, status := range statuses {
		status.ComputeMsgType()
		err = status.Update(tx)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
