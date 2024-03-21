package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"time"

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
	Spec            bool   `help:"Print the blockchain configuration values." default:"false"`
	Genesis         bool   `help:"Get info on the chain genesis and forks." default:"false"`
	ValidatorPubkey string `help:"Get info on the validator with this public key." default:""`
	Peers           bool   `help:"Get info on the validator with this public key." default:"false"`
}

type NewAccountCmd struct {
	WalletDir string `help:"The directory to create the encrypted wallet (keystore) file."`
}

type AccountAddressCmd struct {
	PubKey string `arg:"" help:"The public key of the account."`
}

type AccountBalanceCmd struct {
	Account string `arg:"" help:"The Stratis account to query balance for. 40-byte hex string beginning with 0x"`
	Block   int64  `help:"The block number to retrieve the account balance at. Omit to query the latest block." default:"0"`
}

type AccountCmd struct {
	New     NewAccountCmd     `cmd:"" help:"Create a new Stratis account."`
	Balance AccountBalanceCmd `cmd:"" help:"Get the balance of a Stratis acount."`
}

type ValidatorInfoCmd struct {
	PubKey string `help:"The public key of the validator." default:""`
}

type ValidatorPerfCmd struct {
	Validators []string `help:"A list of validator identifiers: either an index or public keys."`
	StateID    string   `help:"The chain state." default:"head"`
	Start      string   `help:"The chain epoch to start validator data collection."`
	End        string   `help:"The chain epoch to end data collection. Defaults to the most recent epoch." default:""`
	NumEpochs  string   `help:"If either start epoch or end epoch is omitted, indicates how many epochs to collect data from the start or before the end epoch." default:""`
}

type CreateWalletCmd struct {
	Type string `arg:"" help:"The type of wallet to create. Can be nd or hd."`
	Name string `arg:"" help:"The name of the wallet."`
}

type ListWalletCmd struct {
	Type      string `arg:"" help:"The type of wallet to create. Can be nd or hd."`
	Name      string `arg:"" help:"The name of the wallet."`
	WalletDir string `arg:"" help:"The path to the wallet location."`
}
type WalletCmd struct {
	Create CreateWalletCmd `cmd:"createw" help:"Create a wallet."`
	List   ListWalletCmd   `cmd:"createw" help:"Create a wallet."`
}

type ValidatorCmd struct {
	Info ValidatorInfoCmd `cmd:"" help:"Get info on a validator identified by a public key or index."`
	Perf ValidatorPerfCmd `cmd:"" help:"Get info on validator performance."`
}

// Command-line arguments
var CLI struct {
	Debug         bool         `help:"Enable debug mode."`
	Auroria       bool         `help:"Indicates the Auroria testnet should be used. Thhe execution client HTTP API will default to https://auroria.rpc.stratisevm.com/."`
	HttpUrl       string       `help:"The URL of the Stratis execution client HTTP API." default:"https://rpc.stratisevm.com"`
	BeaconHttpUrl string       `help:"The URL of the Stratis consensus client HTTP API." default:"http://localhost:3500"`
	Timeout       int          `help:"Timeout for network operations." default:"120"`
	Ping          PingCmd      `cmd:"" help:"Ping the Stratis node. This verifies your Stratis node is up and the execution and consensus client HTTP APIs are reachable by strac."`
	Info          InfoCmd      `cmd:"" help:"Get information on the Stratis network."`
	Account       AccountCmd   `cmd:"" help:"Work with Stratis accounts."`
	Validator     ValidatorCmd `cmd:"" help:"Get info on Stratis validators."`
	Wallet        WalletCmd    `cmd:"" help:"Work with wallets"`
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
	_ctx, cancel := context.WithTimeout(context.Background(), time.Duration(CLI.Timeout)*time.Second)
	blockchain.Ctx = _ctx
	defer cancel()
	if CLI.Auroria && CLI.HttpUrl == "https://rpc.stratisevm.com" {
		CLI.HttpUrl = "https://auroria.rpc.stratisevm.com/"
	}
	err := blockchain.InitEC(CLI.HttpUrl)
	if err != nil {
		log.Fatalf("error connecting to execution client API at %s: %v", CLI.HttpUrl, err)
	}
	log.Infof("Using execution client API at %v.", CLI.HttpUrl)

	cid, err := blockchain.GetChainID()
	if err != nil {
		log.Fatalf("could not get chain id")
	}

	if CLI.Auroria && cid.Cmp(big.NewInt(205205)) != 0 {
		if cid == big.NewInt(105105) {
			log.Fatalf("auroria testnet specified but execution client is on mainnet")
		} else {
			log.Fatalf("auroria testnet specified but execution client is on chain id %v", cid)
		}
	} else if !CLI.Auroria && cid.Cmp(big.NewInt(105105)) != 0 {
		if cid == big.NewInt(205205) {
			log.Fatalf("mainnet specified but execution client is on auroria testnet")
		} else {
			log.Fatalf("mainnet specified but execution client is on chain id %v", cid)
		}
	}

	if util.Contains(ctx.Args, "info") {
		err := blockchain.InitCC(CLI.BeaconHttpUrl, CLI.Timeout)
		if err != nil {
			log.Fatalf("error connecting to consensus client API at %s: %v", CLI.BeaconHttpUrl, err)
		} else {
			log.Infof("Using consensus client API at %v.", CLI.BeaconHttpUrl)
		}
	}
	ctx.FatalIfErrorf(ctx.Run(&kong.Context{}))
}

func (l *PingCmd) Run(ctx *kong.Context) error {
	return blockchain.Ping()
}

func (l *InfoCmd) Run(ctx *kong.Context) error {
	return blockchain.Info(l.Spec, l.Genesis, l.Peers)
}

func (l *NewAccountCmd) Run(ctx *kong.Context) error {
	return accounts.NewAccount(&l.WalletDir)
}

func (l *AccountBalanceCmd) Run(ctx *kong.Context) error {
	return accounts.BalanceAt(l.Account, l.Block)
}

func (l *ValidatorInfoCmd) Run(ctx *kong.Context) error {
	return validators.Info(l.PubKey)
}

func (l *ValidatorPerfCmd) Run(ctx *kong.Context) error {
	return validators.Perf(l.Validators, l.StateID, l.Start, l.End, l.NumEpochs)
}

func (l *CreateWalletCmd) Run(ctx *kong.Context) error {
	log.Info(l.Type)
	log.Info(l.Name)
	return nil
}

func (l *AccountAddressCmd) Run(ctx *kong.Context) error {
	return accounts.AccountAddress(l.PubKey)
}
