package server

import (
	_ "fmt"
	"github.com/codegangsta/cli"
	"github.com/golang/glog"
	"os"
	"os/signal"
)

type Daemon struct {
	Conf        *BridgeConf
	isBootstrap bool
}

func NewDaemon() *Daemon {
	return &Daemon{
		&BridgeConf{},
		false,
	}
}
func (d *Daemon) Run(ctx *cli.Context) {
	glog.Info("Daemon Starting ...")

	d.isBootstrap = ctx.Bool("bootstrap")

	go ServeAPI(d)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			// TODO clean up work
			os.Exit(0)
		}
	}()
	select {}
}
