package server

import (
	"log"
	"os"

	"github.com/WIZARD-CXY/cxy-sdn/netAgent"
	"github.com/WIZARD-CXY/cxy-sdn/util"
)

const dataDir = "/tmp/cxy/"

type Listener struct{}

var listener Listener

func InitAgent(d *Daemon) error {
	// advance setup server mode
	// ref https://www.consul.io/docs/guides/bootstrapping.html
	err := netAgent.StartAgent(d.isServer, d.expServerNum, d.bindInterface, dataDir)

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
				log.Println("Error joining the cluster")
			}
			log.Println("join to cluster", ip)
			// none-bootstrap node need connect to server leader before ready to work
		case nodeLeave:
			if err := leave(); err != nil {
				log.Println("Error leaving the cluster")
			}

		}
	}
}

var daemon *Daemon

func (l Listener) NotifyNodeUpdate(nType netAgent.NotifyUpdateType, nodeAddr string) {
	if !daemon.isReady {
		daemon.isReady = true
		daemon.readyChan <- true
	}
	if nType == netAgent.NOTIFY_UPDATE_ADD {
		log.Println(nodeAddr, "node joined in")
		myIp, _ := util.MyIP()
		if nodeAddr != myIp {
			// add tunnel to the other node
			AddPeer(nodeAddr)
		}
	} else if nType == netAgent.NOTIFY_UPDATE_DELETE {
		log.Println(nodeAddr, "is leaving, removing tunnel")
		// delete tunnel to nodeAddr
		DeletePeer(nodeAddr)
	}
}

func (l Listener) NotifyKeyUpdate(nType netAgent.NotifyUpdateType, key string, data []byte) {
	// do nothing
}

func (l Listener) NotifyStoreUpdate(nType netAgent.NotifyUpdateType, store string, data map[string][]byte) {
	// do nothing
}
