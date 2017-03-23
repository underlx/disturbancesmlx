package interfaces

import (
	sq "github.com/gbl08ma/squirrel"
)

var sdb sq.StatementBuilderType

func init() {
	sdb = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)
}
