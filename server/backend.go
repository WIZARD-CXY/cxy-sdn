package server

import (
	"fmt"
	"github.com/WIZARD-CXY/cxy-sdn/agent"
	"github.com/WIZARD-CXY/cxy-sdn/util"
	"os"
)

const dataDir = "/tmp/cxy/"

type Listener struct{}

var listener Listener

func InitAgent(bindInterface string, bootstrap bool) error {
	// simple set up
	// if one is started as bootstrap node, start it in serverMode
	err := netAgent.StartAgent(bootstrap, bootstrap, bindInterface, dataDir)

	if err == nil {
		go netAgent.RegisterForNodeUpdates(listener)
	}
	return err
}

func join(addr string) error {
	return netAgent.Join(addr)
}

func leave() error {
	if err := netAgent.Leave(); err != nil {

		return err
	}
	//clean the data storage
	if err := os.RemoveAll(dataDir); err != nil {

		return err
	}
	return nil
}

func nodeHandler(d *Daemon) {
	for {
		context := <-d.clusterChan
		switch context.action {
		case nodeJoin:
			ip := context.param
			if err := join(ip); err != nil {
				fmt.Println("Error joining the cluster")
			}
			fmt.Println("join to cluster master", ip)
			// none-bootstrap node need connect to server leader before ready to work
			d.readyChan <- true
		case nodeLeave:
			if err := leave(); err != nil {
				fmt.Println("Error leaving the cluster")
			}

		}
	}
}

func (l Listener) NotifyNodeUpdate(nType netAgent.NotifyUpdateType, nodeAddr string) {
	if nType == netAgent.NOTIFY_UPDATE_ADD {
		fmt.Println(nodeAddr, "node joined in")
		myIp, _ := util.MyIP()
		if nodeAddr != myIp {
			// add tunnel to the other node
			AddPeer(nodeAddr)
		}
	} else if nType == netAgent.NOTIFY_UPDATE_DELETE {
		fmt.Println(nodeAddr, "is leaving, removing tunnel")
		DeletePeer(nodeAddr)
	}
}

func (l Listener) NotifyKeyUpdate(nType netAgent.NotifyUpdateType, key string, data []byte) {
	// do nothing
}

func (l Listener) NotifyStoreUpdate(nType netAgent.NotifyUpdateType, store string, data map[string][]byte) {
	// do nothing
}
