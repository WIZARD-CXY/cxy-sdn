package main

import (
	"github.com/WIZARD-CXY/cxy-sdn/server"
	"github.com/codegangsta/cli"
	"github.com/golang/glog"
	"os"
	"os/signal"
	"syscall"
)

func init() {
	//to-do
}

func main() {
	app := cli.NewApp()
	app.Name = "cxy-sdn"
	app.Usage = "sdn tool for container cloud platform"
	app.Version = "0.1"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "iface, i",
			Value: "auto",
			Usage: "net Interface to bind, default is auto",
		},
		cli.BoolFlag{
			Name:  "bootstrap, b",
			Usage: "Set --bootstrap for the first socketplane instance being started",
		},
	}
	app.Action = func(c *cli.Context) {
		d := server.NewDaemon()
		d.Run(c)
	}

	glog.Info("Installing signal handlers")
	sig_chan := make(chan os.Signal, 1)
	signal.Notify(sig_chan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sig_chan:
		glog.Info("Exiting")
		// TO-DO clean up
		os.Exit(1)
	}

	app.Run(os.Args)

}
