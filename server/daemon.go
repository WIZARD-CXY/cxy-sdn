package server

import (
	"github.com/codegangsta/cli"
	// "github.com/golang/glog"
	"fmt"
	"os"
	"os/signal"
	"time"
)

type Daemon struct {
	bridgeConf     *BridgeConf
	isBootstrap    bool
	connections    map[string]*Connection // each connection is a connected container, key is containerID
	bindInterface  string
	clusterChan    chan *NodeCtx
	connectionChan chan *ConnectionCtx
	readyChan      chan bool
	Gateways       map[string]struct{}
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
		make(map[string]struct{}, 50),
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

	//start a gorouting to start agent
	go func() {
		d.bindInterface = ctx.String("iface")

		fmt.Printf("Using interface %s\n", d.bindInterface)

		if err := InitAgent(d.bindInterface, d.isBootstrap); err != nil {
			fmt.Println("error in Init netAgent")
		}

		time.Sleep(3 * time.Second)
		if d.isBootstrap {
			d.readyChan <- true
		}
	}()

	// start a goroutine
	go nodeHandler(d)

	go func() {
		if _, err := CreateBridge(); err != nil {
			fmt.Println("Err in create ovs bridge", err.Error())
		}

		//wait data store backend ready
		<-d.readyChan
		fmt.Println("ready to work !")

		if _, err := CreateDefaultNetwork(d.isBootstrap); err != nil {
			fmt.Println("Create cxy network error", err.Error())
		}
		if !d.isBootstrap {
			syncNetwork(d)
		}
	}()

	//start a goroutine to manage connection
	go connHandler(d)

	go monitorNetworkTraffic(d)

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
