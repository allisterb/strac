package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kong"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mbndr/figlet4go"

	"github.com/allisterb/strick/accounts"
	"github.com/allisterb/strick/blockchain"
	"github.com/allisterb/strick/util"
)

type PingCmd struct {
}

type NewAccountCmd struct {
	WalletDir string `help:"The directory to create the encrypted wallet (keystore) file."`
}

type BalanceCmd struct {
	Account string `help:"The Stratis account to query balance for. 40-byte hex string beginning with 0x"`
	Block   int64  `help:"The block number to retrieve the account balance at. Leave out to query the latest block." default:"0"`
}

// Command-line arguments
var CLI struct {
	Debug      bool          `help:"Enable debug mode."`
	HttpUrl    string        `help:"The URL of the Stratis node HTTP API." default:"http://localhost:4545"`
	Timeout    int           `help:"Timeout for network operations." default:"10"`
	Ping       PingCmd       `cmd:"" help:"Ping the Stratis node."`
	NewAccount NewAccountCmd `cmd:"" help:"Create a new Stratis account."`
	Balance    BalanceCmd    `cmd:"" help:"Get the balance of a Stratis account."`
}

var log = logging.Logger("strick/main")

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
	renderStr, _ := ascii.RenderOpts("strick", options)
	fmt.Print(renderStr)
	ctx := kong.Parse(&CLI)
	err := blockchain.Init(CLI.HttpUrl, CLI.Timeout)
	if err != nil {
		log.Fatalf("error connecting to node API at %s: %v", CLI.HttpUrl, err)
	} else {
		ctx.FatalIfErrorf(ctx.Run(&kong.Context{}))
	}
}

func (l *PingCmd) Run(ctx *kong.Context) error {
	return blockchain.Ping()
}

func (l *NewAccountCmd) Run(ctx *kong.Context) error {
	return accounts.NewAccount(&l.WalletDir)
}

func (l *BalanceCmd) Run(ctx *kong.Context) error {
	return accounts.BalanceAt(l.Account, l.Block)
}
