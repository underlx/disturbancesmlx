package discordbot

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
	"github.com/gbl08ma/sqalx"

	altmath "github.com/pkg/math"
)

//SQLSystem handles running SQL statements as bot commands
type SQLSystem struct {
	sync.Mutex
	node  sqalx.Node
	txs   map[uint]sqalx.Node
	curID uint
}

type sqlStatement struct {
	code        string
	returnsRows bool
}

// Setup initializes the SQLSystem and configures a command library with
// SQL-related commands
func (ssys *SQLSystem) Setup(node sqalx.Node, cl *CommandLibrary, privilege Privilege) {
	ssys.node = node
	ssys.txs = make(map[uint]sqalx.Node)
	cl.Register(NewCommand("sqlro", ssys.handleRO).WithSkipArgParsing(true).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("sqlrw", ssys.handleRW).WithSkipArgParsing(true).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("sqlbegin", ssys.handleSQLbegin).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("sqlon", ssys.handleRunOn).WithSkipArgParsing(true).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("sqlcommit", ssys.handleCommit).WithRequirePrivilege(privilege))
	cl.Register(NewCommand("sqlrollback", ssys.handleRollback).WithRequirePrivilege(privilege))
}

func (ssys *SQLSystem) handleRO(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	tx, err := ssys.node.Beginx()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ opening transaction: "+err.Error())
		return
	}
	ssys.runOnTx(s, m, tx, args[0])
	err = tx.Rollback()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ rollbacking transaction: "+err.Error())
	}
}

func (ssys *SQLSystem) handleRW(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	tx, err := ssys.node.Beginx()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ opening transaction: "+err.Error())
		return
	}
	ssys.runOnTx(s, m, tx, args[0])
	err = tx.Commit()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ commiting transaction: "+err.Error())
	}
}

func (ssys *SQLSystem) handleSQLbegin(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	tx, err := ssys.node.Beginx()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ opening transaction: "+err.Error())
		return
	}
	ssys.Lock()
	defer ssys.Unlock()

	ssys.txs[ssys.curID] = tx
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("📸 tx %d", ssys.curID))
	ssys.curID++
}

func (ssys *SQLSystem) handleRunOn(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	words := strings.Fields(args[0])
	txID, err := strconv.ParseUint(words[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 tx ID")
		return
	}

	ssys.Lock()
	defer ssys.Unlock()
	tx, ok := ssys.txs[uint(txID)]
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖 tx ID")
		return
	}
	startLen := altmath.MinInt(len(args[0]), len(words[0])+1)
	ssys.runOnTx(s, m, tx, args[0][startLen:])
}

func (ssys *SQLSystem) handleCommit(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	txID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 tx ID")
		return
	}
	ssys.Lock()
	defer ssys.Unlock()
	tx, ok := ssys.txs[uint(txID)]
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖 tx ID")
		return
	}

	delete(ssys.txs, uint(txID))

	err = tx.Commit()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ commiting transaction: "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("💾 tx %d", txID))
}

func (ssys *SQLSystem) handleRollback(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if len(args) < 1 {
		s.ChannelMessageSend(m.ChannelID, "🆖 missing arguments")
		return
	}
	txID, err := strconv.ParseUint(args[0], 10, 32)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "🆖 tx ID")
		return
	}
	ssys.Lock()
	defer ssys.Unlock()
	tx, ok := ssys.txs[uint(txID)]
	if !ok {
		s.ChannelMessageSend(m.ChannelID, "🆖 tx ID")
		return
	}

	delete(ssys.txs, uint(txID))

	err = tx.Rollback()
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "❌ rolling back transaction: "+err.Error())
		return
	}
	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("🔙 tx %d", txID))
}

func (ssys *SQLSystem) rawArgToStatements(arg0 string) []sqlStatement {
	arg0 = strings.Replace(arg0, "```sql", "", -1)
	arg0 = strings.Replace(arg0, "```", "", -1)
	arg0 = strings.Replace(arg0, "\\`\\`\\`", "```", -1)
	stmts := strings.Split(arg0, ";")
	sqlstmts := []sqlStatement{}
	for _, stmt := range stmts {
		stmt = strings.TrimSpace(stmt)
		if len(stmt) == 0 {
			continue
		}
		sqlstmts = append(sqlstmts, sqlStatement{
			code:        stmt,
			returnsRows: strings.HasPrefix(strings.ToLower(stmt), "select"),
		})
	}
	return sqlstmts
}

func (ssys *SQLSystem) runOnTx(s *discordgo.Session, m *discordgo.MessageCreate, tx sqalx.Node, arg0 string) {
	stmts := ssys.rawArgToStatements(arg0)
	for i, stmt := range stmts {
		output := fmt.Sprintf("```sql\n%d> %s;```", i+1, stmt.code)
		if stmt.returnsRows {
			rows, err := tx.Query(stmt.code)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ on query number %d: %s", i+1, err.Error()))
				return
			}

			cols, err := rows.Columns()
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, "❌ getting columns: "+err.Error())
				rows.Close()
				return
			}

			rawResult := make([][]byte, len(cols))
			resultRows := [][]string{}

			dest := make([]interface{}, len(cols)) // A temporary interface{} slice
			for i := range rawResult {
				dest[i] = &rawResult[i] // Put pointers to each string in the interface slice
			}

			longestLengthsPerColumn := make([]int, len(cols))
			for i, col := range cols {
				longestLengthsPerColumn[i] = len(col)
			}

			for rows.Next() {
				err = rows.Scan(dest...)
				if err != nil {
					s.ChannelMessageSend(m.ChannelID, "❌ scanning row: "+err.Error())
					rows.Close()
					return
				}

				result := make([]string, len(cols))

				for i, raw := range rawResult {
					if raw == nil {
						result[i] = "✴"
					} else {
						result[i] = string(raw)
					}
					if len(result[i]) > 50 && len(cols) > 2 {
						result[i] = result[i][:49] + "…"
					}
					longestLengthsPerColumn[i] = altmath.Max(longestLengthsPerColumn[i], len(result[i]))
				}

				resultRows = append(resultRows, result)
			}

			if err := rows.Err(); err != nil {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ on statement number %d: %s", i+1, err.Error()))
				rows.Close()
				return
			}

			output += "```"
			titlerow := ""
			for i, col := range cols {
				if i > 0 {
					titlerow += " | "
				}
				titlerow += fmt.Sprintf("%-"+fmt.Sprintf("%d", longestLengthsPerColumn[i])+"s", col)
			}
			output += titlerow + "\n" + strings.Repeat("-", len(titlerow)) + "\n"

			printedRows := 0
			for _, row := range resultRows {
				rowstr := ""
				for i := range cols {
					if i > 0 {
						rowstr += " | "
					}
					rowstr += fmt.Sprintf("%-"+fmt.Sprintf("%d", longestLengthsPerColumn[i])+"s", row[i])
				}
				if len(output)+len(rowstr) > 1970 {
					break
				}
				output += rowstr + "\n"
				printedRows++
			}

			output += fmt.Sprintf("``` %d rows (%d omitted)\n", len(resultRows), len(resultRows)-printedRows)
			rows.Close()
		} else {
			result, err := tx.Exec(stmt.code)
			if err != nil {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("❌ on statement number %d: %s", i+1, err.Error()))
				return
			}
			rowsAffected, err := result.RowsAffected()
			if err != nil {
				output += "❌ retrieving rows affected: " + err.Error()
			} else {
				output += fmt.Sprintf("%d rows affected", rowsAffected)
			}
		}
		s.ChannelMessageSend(m.ChannelID, output)
	}
}
