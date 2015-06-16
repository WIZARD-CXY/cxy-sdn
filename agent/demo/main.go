package main

import (
	"fmt"
	"time"

	"flag"
	"github.com/WIZARD-CXY/netAgent"
	"github.com/golang/glog"
	"os"
	"os/signal"
)

const dataDir = "/tmp/test"

type Listener struct {
}

func (e Listener) NotifyNodeUpdate(Type netAgent.NotifyUpdateType, nodeName string) {
	fmt.Println("Node update", Type, nodeName)
}

func (e Listener) NotifyKeyUpdate(Type netAgent.NotifyUpdateType, key string, value []byte) {
	fmt.Println("Key update", Type, key, string(value))
}
func (e Listener) NotifyStoreUpdate(Type netAgent.NotifyUpdateType, store string, data map[string][]byte) {

}
func main() {
	isBootstrap := flag.Bool("b", false, "bootstrap")
	isServerMode := flag.Bool("s", false, "serverMode")
	netInterface := flag.String("i", "eth0", "bind interface")
	serverAddr := flag.String("sa", "10.10.105.2", "server addr")

	flag.Parse()
	err := netAgent.StartAgent(*isServerMode, *isBootstrap, *netInterface, dataDir)

	if !*isBootstrap {
		netAgent.Join(*serverAddr)
	}

	if err != nil {
		glog.Fatalf("start agent failed")
	}
	listener := Listener{}
	go netAgent.RegisterForNodeUpdates(listener)
	go netAgent.RegisterForKeyUpdates("haha", "test", listener)

	keyUpdates("test")

	netAgent.Delete("haha", "test")
	go netAgent.RegisterForKeyUpdates("haha", "test2", listener)
	keyUpdates("test2")
	keyUpdates("test")

	sig_chan := make(chan os.Signal, 1)
	signal.Notify(sig_chan, os.Interrupt)
	for sig := range sig_chan {
		if sig == os.Interrupt {
			netAgent.Leave()
			break
		}
	}

}

//Random Key updates
func keyUpdates(key string) {
	currVal, _, _ := netAgent.Get("network", key)
	newVal := make([]byte, len(currVal))
	newVal = []byte("value1")
	netAgent.Put("haha", key, newVal, currVal)
	time.Sleep(time.Second * 2)
	updArray := []byte("value2")
	netAgent.Put("haha", key, updArray, newVal)
	time.Sleep(time.Second * 2)
}
