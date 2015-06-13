package server

import (
	_ "fmt"
	"github.com/codegangsta/cli"
	// "github.com/golang/glog"
	"fmt"
	"os"
	"os/signal"
)

type Daemon struct {
	bridgeConf    *BridgeConf
	isBootstrap   bool
	connections   map[string]*Connection
	bindInterface string
	clusterChan   chan *ClusterInfo
}

type ClusterInfo struct {
	param  string
	action int
}

const (
	nodeJoin = iota
	nodeLeave
)

func NewDaemon() *Daemon {
	return &Daemon{
		&BridgeConf{},
		false,
		map[string]*Connection{},
		"",
		make(chan *ClusterInfo),
	}
}
func (d *Daemon) Run(ctx *cli.Context) {
	// glog.Info("Daemon Starting ...")

	d.isBootstrap = ctx.Bool("bootstrap")

	// use for netns
	if err := os.Mkdir("/var/run/netns", 0777); err != nil {
		fmt.Println("mkdir /var/run/netns failed", err)
	}

	// start a goroutine to serve api
	go ServeApi(d)

	//start a gorouting to start netAgent
	go func() {
		d.bindInterface = ctx.String("iface")

		fmt.Printf("Using interface %s\n", d.bindInterface)

		if err := InitAgent(d.bindInterface, d.isBootstrap); err != nil {
			fmt.Println("error in Init netAgent")
		}
	}()

	// start a goroutine
	go manageNode(d)

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

func manageNode(d *Daemon) {
	for {
		context := <-d.clusterChan
		switch context.action {
		case nodeJoin:
			ip := context.param
			if err := join(ip); err != nil {
				fmt.Println("Error joining the cluster")
			}
		case nodeLeave:
			if err := leave(); err != nil {
				fmt.Println("Error leaving the cluster")
			}

		}
	}
}
