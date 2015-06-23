package server

import (
	"github.com/codegangsta/cli"
	// "github.com/golang/glog"
	"fmt"
	"os"
	"os/signal"
)

type Daemon struct {
	bridgeConf     *BridgeConf
	isBootstrap    bool
	connections    map[string]*Connection
	bindInterface  string
	clusterChan    chan *NodeCtx
	connectionChan chan *ConnectionCtx
	readyChan      chan bool
}

type NodeCtx struct {
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
		make(map[string]*Connection, 50),
		"",
		make(chan *NodeCtx),
		make(chan *ConnectionCtx),
		make(chan bool),
	}
}
func (d *Daemon) Run(ctx *cli.Context) {
	d.isBootstrap = ctx.Bool("bootstrap")

	// set up dir use for netns
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
	go nodeHandler(d)

	go func() {
		if !d.isBootstrap {
			fmt.Println("None bootstrap node , wait for joining the cluster")
			<-d.readyChan
			fmt.Println("None bootstrap node , joined to the cluster")
		}

		if _, err := CreateBridge(); err != nil {
			fmt.Println("Err in create ovs bridge", err.Error())
		}

		if _, err := CreateDefaultNetwork(); err != nil {
			fmt.Println("Create Default network error")
		}
	}()

	go connHandler(d)

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
