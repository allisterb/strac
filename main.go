package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mbndr/figlet4go"

	"github.com/allisterb/strac/accounts"
	"github.com/allisterb/strac/blockchain"
	"github.com/allisterb/strac/util"
	"github.com/allisterb/strac/validators"
)

type PingCmd struct {
}

type InfoCmd struct {
	Genesis         bool   `help:"Get info on the chain genesis and forks." default:"false"`
	ValidatorPubkey string `help:"Get info on the validator with this public key." default:""`
	Peers           bool   `help:"Get info on the validator with this public key." default:"false"`
}

type NewAccountCmd struct {
	WalletDir string `help:"The directory to create the encrypted wallet (keystore) file."`
}

type BalanceCmd struct {
	Account string `help:"The Stratis account to query balance for. 40-byte hex string beginning with 0x"`
	Block   int64  `help:"The block number to retrieve the account balance at. Omit to query the latest block." default:"0"`
}

type ValidatorsCmd struct {
	Validators []string `arg:"" help:"existingfile"`
	StateID    string   `help:"The chain state." default:"head"`
	Epoch      string   `help:"The chain epoch." default:""`
}

// Command-line arguments
var CLI struct {
	Debug         bool          `help:"Enable debug mode."`
	HttpUrl       string        `help:"The URL of the Stratis node HTTP API." default:"http://localhost:8545"`
	BeaconHttpUrl string        `help:"The URL of the Stratis beacon client HTTP API." default:"http://localhost:3500"`
	Timeout       int           `help:"Timeout for network operations." default:"10"`
	Ping          PingCmd       `cmd:"" help:"Ping the Stratis node."`
	Info          InfoCmd       `cmd:"" help:"Get information on the Stratis network."`
	NewAccount    NewAccountCmd `cmd:"" help:"Create a new Stratis account."`
	Balance       BalanceCmd    `cmd:"" help:"Get the balance of a Stratis account."`
	Validators    ValidatorsCmd `cmd:"" help:"Get info on validators."`
}

var log = logging.Logger("strac/main")

func init() {
	if os.Getenv("GOLOG_LOG_LEVEL") == "" {
		logging.SetAllLoggers(logging.LevelInfo)
	}
	if util.Contains(os.Args, "--debug") {
		logging.SetAllLoggers(logging.LevelDebug)
	}
}

func main() {
	if util.Contains(os.Args, "--debug") {
		log.Info("Debug mode enabled.")
	}
	ascii := figlet4go.NewAsciiRender()
	options := figlet4go.NewRenderOptions()
	options.FontColor = []figlet4go.Color{
		figlet4go.ColorCyan,
		figlet4go.ColorMagenta,
		figlet4go.ColorYellow,
	}
	renderStr, _ := ascii.RenderOpts("strac", options)
	fmt.Print(renderStr)
	ctx := kong.Parse(&CLI)
	err := blockchain.Init(CLI.HttpUrl, CLI.BeaconHttpUrl, CLI.Timeout)
	if err != nil {
		log.Fatalf("error connecting to execution client API and consensus client at %s, %s: %v", CLI.HttpUrl, CLI.BeaconHttpUrl)
	} else {
		ctx.FatalIfErrorf(ctx.Run(&kong.Context{}))
	}
}

func (l *PingCmd) Run(ctx *kong.Context) error {
	return blockchain.Ping()
}

func (l *InfoCmd) Run(ctx *kong.Context) error {
	return blockchain.Info(l.Genesis, l.ValidatorPubkey, l.Peers)
}

func (l *NewAccountCmd) Run(ctx *kong.Context) error {
	return accounts.NewAccount(&l.WalletDir)
}

func (l *BalanceCmd) Run(ctx *kong.Context) error {
	return accounts.BalanceAt(l.Account, l.Block)
}

func (l *ValidatorsCmd) Run(ctx *kong.Context) error {
	if err := validators.Init(); err != nil {
		return err
	}
	_, err := validators.Summary(l.Validators, l.StateID, l.Epoch)
	return err
}
