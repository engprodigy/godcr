package cli

import (
	"context"
	"fmt"
	"github.com/raedahgroup/dcrcli/app"
	"github.com/raedahgroup/dcrcli/cli/terminalprompt"
	"os"
	"strings"
)

// createWallet creates a new wallet if one doesn't already exist using the WalletMiddleware provided
func createWallet(ctx context.Context, walletMiddleware app.WalletMiddleware) (err error) {
	// first check if wallet already exists
	walletExists, err := walletMiddleware.WalletExists()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error checking %s wallet: %s\n", walletMiddleware.NetType(), err.Error())
		return
	}
	if walletExists {
		netType := strings.Title(walletMiddleware.NetType())
		fmt.Fprintf(os.Stderr, "%s wallet already exists", netType)
		return fmt.Errorf("wallet already exists")
	}

	// ask user to enter passphrase twice
	passphrase, err := terminalprompt.RequestInputSecure("Enter private passphrase for new wallet", terminalprompt.EmptyValidator)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %s", err.Error())
		return
	}
	confirmPassphrase, err := terminalprompt.RequestInputSecure("Confirm passphrase", terminalprompt.EmptyValidator)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %s\n", err.Error())
		return
	}
	if passphrase != confirmPassphrase {
		fmt.Fprintln(os.Stderr, "Passphrases do not match")
		return fmt.Errorf("passphrases do not match")
	}

	// get seed and display to user
	seed, err := walletMiddleware.GenerateNewWalletSeed()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating seed for new wallet: %s\n", err)
		return
	}
	displayWalletSeed(seed)

	// ask user to back seed up, only proceed after user does so
	backupPrompt := `Enter "OK" to continue. This assumes you have stored the seed in a safe and secure location`
	backupValidator := func(userResponse string) error {
		userResponse = strings.TrimSpace(userResponse)
		userResponse = strings.Trim(userResponse, `"`)
		if strings.EqualFold("OK", userResponse) {
			return nil
		} else {
			return fmt.Errorf("invalid response, try again")
		}
	}
	_, err = terminalprompt.RequestInput(backupPrompt, backupValidator)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %s", err.Error())
		return
	}

	// user entered "OK" in last prompt, finalize wallet creation
	err = walletMiddleware.CreateWallet(passphrase, seed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating wallet: %s", err.Error())
		return
	}
	fmt.Println("Your wallet has been created successfully")

	// perform first blockchain sync after creating wallet
	return syncBlockChain(ctx, walletMiddleware)
}

// openWallet is called whenever an action to be executed requires wallet to be loaded
// notifies the program to exit if wallet doesn't exist or some other error occurs by returning a non-nil error
//
// this method may stall until previous dcrcli instances are closed (especially in cases of multiple mobilewallet instances)
// hence the need for ctx, so user can cancel the operation if it's taking too long
func openWallet(ctx context.Context, walletMiddleware app.WalletMiddleware) error {
	// notify user of the current operation so if takes too long, they have an idea what the cause is
	fmt.Println("Looking for wallets...")

	var err error
	var errMsg string
	var noWalletFound bool
	loadWalletDone := make(chan bool)

	go func() {
		defer func() {
			loadWalletDone <- true
		}()

		var walletExists bool
		walletExists, err = walletMiddleware.WalletExists()
		if err != nil {
			errMsg = fmt.Sprintf("Error checking %s wallet", walletMiddleware.NetType())
			return
		}

		if !walletExists {
			noWalletFound = true
			return
		}

		err = walletMiddleware.OpenWallet()
		if err != nil {
			errMsg = fmt.Sprintf("Failed to open %s wallet", walletMiddleware.NetType())
		}
	}()

	select {
	case <-loadWalletDone:
		if noWalletFound {
			return attemptToCreateWallet(ctx, walletMiddleware)
		}

		if errMsg != "" {
			fmt.Fprintln(os.Stderr, errMsg)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		return err

	case <-ctx.Done():
		return ctx.Err()
	}
}

func attemptToCreateWallet(ctx context.Context, walletMiddleware app.WalletMiddleware) error {
	createWalletPrompt := "No wallet found. Would you like to create one now? [y/N]"
	validateUserResponse := func(userResponse string) error {
		userResponse = strings.TrimSpace(userResponse)
		userResponse = strings.Trim(userResponse, `"`)
		if userResponse == "" || strings.EqualFold("y", userResponse) || strings.EqualFold("N", userResponse) {
			return nil
		} else {
			return fmt.Errorf("invalid option, try again")
		}
	}

	userResponse, err := terminalprompt.RequestInput(createWalletPrompt, validateUserResponse)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading your response: %s", err.Error())
		return err
	}

	if userResponse == "" || strings.EqualFold("N", userResponse) {
		fmt.Println("Maybe later. Bye.")
		return fmt.Errorf("Wallet doesn't exist")
	}

	return createWallet(ctx, walletMiddleware)
}

// syncBlockChain uses the WalletMiddleware provided to download block updates
// this is a long running operation, listen for ctx.Done and stop processing
func syncBlockChain(ctx context.Context, walletMiddleware app.WalletMiddleware) error {
	syncDone := make(chan error)
	go func() {
		syncListener := &app.BlockChainSyncListener{
			SyncStarted: func() {
				fmt.Println("Blockchain sync started")
			},
			SyncEnded: func(err error) {
				if err == nil {
					fmt.Println("Blockchain synced successfully")
				} else {
					fmt.Fprintf(os.Stderr, "Blockchain sync completed with error: %s", err.Error())
				}
				syncDone <- err
			},
			OnHeadersFetched:    func(percentageProgress int64) {}, // in cli mode, sync updates are logged to terminal, no need to act on this update alert
			OnDiscoveredAddress: func(state string) {},             // in cli mode, sync updates are logged to terminal, no need to act on update alert
			OnRescanningBlocks:  func(percentageProgress int64) {}, // in cli mode, sync updates are logged to terminal, no need to act on update alert
		}

		err := walletMiddleware.SyncBlockChain(syncListener, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Blockchain sync failed to start. %s", err.Error())
			syncDone <- err
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-syncDone:
		return err
	}
}
