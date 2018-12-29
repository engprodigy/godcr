package commands

import (
	"github.com/raedahgroup/dcrcli/cli/termio"
	"github.com/raedahgroup/dcrcli/walletrpcclient"
)

// HistoryCommand enables the user view their transaction history.
type HistoryCommand struct {
	CommanderStub
}

// Execute runs the `history` command.
func (h HistoryCommand) Run(client *walletrpcclient.Client, args []string) error {
	transactions, err := client.GetTransactions()
	if err != nil {
		return err
	}

	columns := []string{
		"Date",
		"Amount (DCR)",
		"Direction",
		"Hash",
		"Type",
	}
	rows := make([][]interface{}, len(transactions))

	for i, tx := range transactions {
		rows[i] = []interface{}{
			tx.FormattedTime,
			tx.Amount,
			tx.Direction,
			tx.Hash,
			tx.Type,
		}
	}

	termio.PrintTabularResult(termio.StdoutWriter, columns, rows)
	return nil
}
