package server

import (
	"github.com/WIZARD-CXY/netAgent"
	"github.com/golang/glog"
	"os"
)

const dataDir = "/tmp/cxy/"

var listener netAgentListener

func Init(bindInterface string, bootstrap bool) error {
	// simple set up
	// if one is started as bootstrap node, start it in serverMode
	err := netAgent.StartAgent(serverMode, bootstrap, bindInterface, dataDir)

	if err == nil {
		go netAgent.RegisterForNodeUpdates(listener)
	}
	return err
}

func JoinCluster(addr string) error {
	return netAgent.join(addr)
}

func LeaveDataStore() error {
	if err := netAgent.Leave(); err != nil {
		glog.Error(err)
		return err
	}

	//clean the data storage
	if err := os.RemoveAll(dataDir); err != nil {
		glog.Error(err)
		return err
	}

	return nil
}

// just empty
type listener struct Listener{

}

func (l Listener) NotifyNodeUpdate(nType netAgent.NotifyUpdateType, nodeAddr string){
	if nType == netAgent.NOTIFY_UPDATE_ADD{
		glog.Infof("New node %s joined in", nodeAddr)
		AddPeer(nodeAddr)
	}else if nType == netAgent.NOTIFY_UPDATE_DELETE{
		glog.Infof("Node %s left", nodeAddr)
		DeletePeer(nodeAddr)
	}
}

func (l Listener) NotifyKeyUpdate(nType netAgent.NotifyUpdateType, key string, data []byte){
	// do nothing
}

func (l Listener) NotifyStoreUpdate(nType netAgent.NotifyUpdateType, store string, data map[string][]byte){
	// do nothing
}
