package server

import (
	_ "fmt"
	"github.com/codegangsta/cli"
	// "github.com/golang/glog"
	"os"
	"os/signal"
)

type Daemon struct {
	bridgeConf  *BridgeConf
	isBootstrap bool
	connections map[string]*Connection
}

func NewDaemon() *Daemon {
	return &Daemon{
		&BridgeConf{},
		false,
		map[string]*Connection{},
	}
}
func (d *Daemon) Run(ctx *cli.Context) {
	// glog.Info("Daemon Starting ...")

	d.isBootstrap = ctx.Bool("bootstrap")

	go ServeAPI(d)

	sig_chan := make(chan os.Signal, 1)
	signal.Notify(sig_chan, os.Interrupt)
	go func() {
		for _ = range sig_chan {
			// TODO clean up work
			os.Exit(0)
		}
	}()
	select {}
}
