package commands

import (
	"fmt"

	"github.com/raedahgroup/dcrcli/cli/termio"
	"github.com/raedahgroup/dcrcli/walletrpcclient"
	qrcode "github.com/skip2/go-qrcode"
)

// ReceiveCommand generates and address for a user to receive DCR.
type ReceiveCommand struct {
	CommanderStub
	Args struct {
		Account string `positional-arg-name:"account"`
	} `positional-args:"yes"`
}

// Run runs the `receive` command.
func (r ReceiveCommand) Run(client *walletrpcclient.Client, args []string) error {
	var accountNumber uint32
	// if no account name was passed in
	if r.Args.Account == "" {
		// display menu options to select account
		var err error
		accountNumber, err = termio.GetSendSourceAccount(client)
		if err != nil {
			return err
		}
	} else {
		// if an account name was passed in e.g. ./dcrcli receive default
		// get the address corresponding to the account name and use it
		var err error
		accountNumber, err = client.AccountNumber(r.Args.Account)
		if err != nil {
			return fmt.Errorf("Error fetching account number: %s", err.Error())
		}
	}

	receiveResult, err := client.Receive(accountNumber)
	if err != nil {
		return err
	}

	qr, err := qrcode.New(receiveResult.Address, qrcode.Medium)
	if err != nil {
		return fmt.Errorf("Error generating QR Code: %s", err.Error())
	}

	columns := []string{
		"Address",
		"QR Code",
	}
	rows := [][]interface{}{
		[]interface{}{
			receiveResult.Address,
			qr.ToString(true),
		},
	}
	termio.PrintTabularResult(termio.StdoutWriter, columns, rows)
	return nil
}
