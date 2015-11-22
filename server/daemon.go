package server

import (
	"fmt"
	"github.com/codegangsta/cli"
	"os"
	"os/signal"
	"sync"
	"time"
)

// concurrent-safe map
type SafeMap struct {
	sync.RWMutex
	rm map[string]interface{}
}

func NewSafeMap() *SafeMap {
	return &SafeMap{
		rm: make(map[string]interface{}),
	}
}

func (m *SafeMap) Get(k string) interface{} {
	m.RLock()
	defer m.RUnlock()

	if val, ok := m.rm[k]; ok {
		return val
	}
	return nil
}

// returns false if k is already in the sm and v is same with the old value
func (m *SafeMap) Set(k string, v interface{}) bool {
	m.Lock()
	defer m.Unlock()

	if val, ok := m.rm[k]; !ok {
		m.rm[k] = v
	} else if val != v {
		m.rm[k] = v
	} else {
		return false
	}
	return true
}

func (m *SafeMap) Check(k string) bool {
	m.RLock()
	defer m.RUnlock()

	_, ok := m.rm[k]

	return ok
}

func (m *SafeMap) Delete(k string) {
	m.Lock()
	defer m.Unlock()
	delete(m.rm, k)
}

type Daemon struct {
	bridgeConf     *BridgeConf
	isBootstrap    bool
	connections    *SafeMap // each connection is a connected container, key is containerID
	bindInterface  string
	clusterChan    chan *NodeCtx
	connectionChan chan *ConnectionCtx
	readyChan      chan bool
	Gateways       map[string]struct{} //network set
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
		NewSafeMap(),
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

	// start a gorouting to start agent
	go func() {
		d.bindInterface = ctx.String("iface")

		fmt.Printf("Using interface %s\n", d.bindInterface)

		if err := InitAgent(d.bindInterface, d.isBootstrap); err != nil {
			fmt.Println("error in Init netAgent")
		}

		// wait a while for agent to fully start
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
