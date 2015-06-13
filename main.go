package main

import (
	"github.com/WIZARD-CXY/cxy-sdn/server"
	"github.com/codegangsta/cli"
	"os"
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
			Value: "eth0",
			Usage: "Network Interface to bind, default is eth0",
		},
		cli.BoolFlag{
			Name:  "bootstrap, b",
			Usage: "--bootstrap/-b for the first instance being started",
		},
	}

	app.Action = func(c *cli.Context) {
		d := server.NewDaemon()
		d.Run(c)
	}

	app.Run(os.Args)
}
