package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mbndr/figlet4go"

	"github.com/allisterb/strick/blockchain"
	"github.com/allisterb/strick/util"
)

type PingCmd struct {
}

// Command-line arguments
var CLI struct {
	Debug  bool    `help:"Enable debug mode."`
	RPCUrl string  `help:"The URL of the Stratis node RPC." default:"http://localhost:4545"`
	Ping   PingCmd `cmd:"" help:"Ping the Stratis node."`
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
	ctx.FatalIfErrorf(ctx.Run(&kong.Context{}))
}

func (l *PingCmd) Run(ctx *kong.Context) error {
	cctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	err := blockchain.Ping(cctx, CLI.RPCUrl)
	if err != nil {
		log.Errorf("Error pinging node at %v: %v", CLI.RPCUrl, err)
	}
	cancel()
	return nil
}
