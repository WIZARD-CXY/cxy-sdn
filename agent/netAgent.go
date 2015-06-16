// Embedded net Agent

package netAgent

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/command"
	"github.com/hashicorp/consul/watch"
	"github.com/mitchellh/cli"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"
)

func StartAgent(serverMode bool, bootstrap bool, bindInterface string, dataDir string) error {
	bindAddr := ""

	if bindInterface != "" {
		netInterface, err := net.InterfaceByName(bindInterface)
		if err != nil {
			glog.Fatalf("Error : %v", err)
			return err
		}

		addrs, err := netInterface.Addrs()

		if err == nil {
			for _, addr := range addrs {
				ip, _, _ := net.ParseCIDR(addr.String())

				if ip != nil && ip.To4() != nil {
					bindAddr = ip.To4().String()
				}
			}
		}

	}

	errChan := make(chan int)

	watchForExistingRegisteredUpdates()

	//go RegisterForNodeUpdates()
	go startConsul(serverMode, bootstrap, bindAddr, dataDir, errChan)

	select {
	case <-errChan:
		return errors.New("Error start consul agent")
	case <-time.After(time.Second * 5):
	}
	return nil
}

func startConsul(serverMode bool, bootstrap bool, bindAddress string, dataDir string, eCh chan int) {
	args := []string{"agent", "-data-dir", dataDir}

	if serverMode {
		args = append(args, "-server")
	}

	if bootstrap {
		args = append(args, "-bootstrap-expect=1")
	}

	if bindAddress != "" {
		args = append(args, "-bind")
		args = append(args, bindAddress)
		args = append(args, "-advertise")
		args = append(args, bindAddress)
	}
	args = append(args)

	ret := Execute(args...)

	eCh <- ret
}

// Execute function is borrowed from Consul's main.go
func Execute(args ...string) int {
	cli := &cli.CLI{
		Args:     args,
		Commands: Commands,
		HelpFunc: cli.BasicHelpFunc("consul"),
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
		return 1
	}

	return exitCode
}

// Node operation related

type Node struct {
	Name    string `json:"Name,omitempty"`
	Address string `json:"Addr,ommitempty"`
}

const CONSUL_CATALOG_BASE_URL = "http://localhost:8500/v1/catalog/"

func Join(addr string) error {
	ret := Execute("join", addr)

	if ret != 0 {
		glog.Errorf("Error (%d) joining %s with consul peers", ret, addr)
		return errors.New("Error joining the cluster")
	}

	return nil
}

func Leave() error {
	ret := Execute("leave")

	if ret != 0 {
		glog.Errorf("Error leaving consul cluster")
		return errors.New("Error leaving consul cluster")
	}
	return nil
}

func getNodes() ([]Node, error) {
	url := CONSUL_CATALOG_BASE_URL + "nodes"

	resp, err := http.Get(url)
	if err != nil {
		glog.Errorf("Error (%v) get %s", err, url)
		return nil, errors.New("Get nodes failed")
	}

	defer resp.Body.Close()

	glog.Infof("Get %s for %s\n", resp.Status, url)

	var nodes []Node

	err = json.NewDecoder(resp.Body).Decode(nodes)

	if err != nil {
		glog.Errorf("getNodes failed error decoding")
		return nil, errors.New("Decode error in getNodes")
	}

	return nodes, nil

}

// K/V store

const CONSUL_KV_BASE_URL = "http://localhost:8500/v1/kv/"

type KVRespBody struct {
	CreateIndex int    `json:"CreateIndex,omitempty"`
	ModifyIndex int    `json:"ModifyIndex,omitempty"`
	Key         string `json:"Key,omitempty"`
	Flags       int    `json:Flags,omitempty"`
	Value       string `json:Value,omitempty"`
}

// 1st return value as []byte
// 2nd return its ModifyIndex
// 3rd return ok or not
func Get(store string, key string) ([]byte, int, bool) {
	url := CONSUL_KV_BASE_URL + store + "/" + key

	resp, err := http.Get(url)

	if err != nil {
		glog.Errorf("Error (%v) in Get for %s\n", err, url)
		return nil, 0, false
	}

	defer resp.Body.Close()

	glog.Infof("Status of Get %s for %s", resp.Status, url)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var jsonBody []KVRespBody

		err = json.NewDecoder(resp.Body).Decode(&jsonBody)

		existingValue, err := b64.StdEncoding.DecodeString(jsonBody[0].Value)

		if err != nil {
			return nil, jsonBody[0].ModifyIndex, false
		}

		return existingValue, jsonBody[0].ModifyIndex, true

	} else {
		return nil, 0, false
	}
}

// get all key-value pairs in one store from backend
func GetAll(store string) ([][]byte, []int, bool) {
	url := CONSUL_KV_BASE_URL + store + "?recursive"

	resp, err := http.Get(url)
	defer resp.Body.Close()

	if err != nil {
		glog.Infof("Error in Get all KV %v", store)
	}
	fmt.Printf("Status of Get %s for %s\n", resp.Status, url)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		var jsonBody []KVRespBody

		values := make([][]byte, 0)
		indexes := make([]int, 0)

		err = json.NewDecoder(resp.Body).Decode(&jsonBody)

		for _, body := range jsonBody {
			existingVal, _ := b64.StdEncoding.DecodeString(body.Value)
			values = append(values, existingVal)
			indexes = append(indexes, body.ModifyIndex)
		}

		return values, indexes, true
	} else {
		return nil, nil, false
	}

}

const (
	OK = iota
	OUTDATED
	ERROR
)

// return val indicate the error type
// need old val as 4-th param
func Put(store string, key string, value []byte, oldVal []byte) int {

	existingVal, casIndex, ok := Get(store, key)

	if ok && !bytes.Equal(oldVal, existingVal) {
		return OUTDATED
	}

	url := CONSUL_KV_BASE_URL + store + "/" + key + "?cas=" + strconv.Itoa(casIndex)
	glog.Infof("Updating KV pair for %s %s %s %d", url, key, value, casIndex)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(value))

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		glog.Errorf("Error creating KV pair for %s", key)
		return ERROR
	}

	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	if string(body) == "false" {
		return ERROR
	}

	return OK

}

// return val indicate the error type
func Delete(store string, key string) int {
	url := fmt.Sprintf("%s%s/%s", CONSUL_KV_BASE_URL, store, key)

	glog.Infof("Deleting KV pair for %s", url)

	req, err := http.NewRequest("DELETE", url, nil)

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		glog.Errorf("Error deleting KV pair %d", key)
		return ERROR
	}

	defer resp.Body.Close()
	return OK
}

// Watch related

const (
	NOTIFY_UPDATE_ADD = iota
	NOTIFY_UPDATE_MODIFY
	NOTIFY_UPDATE_DELETE
)

type NotifyUpdateType int

const (
	WATCH_TYPE_NODE = iota
	WATCH_TYPE_KEY
	WATCH_TYPE_STORE
	WATCH_TYPE_EVENT
)

type WatchType int

type watchData struct {
	listeners  map[string][]Listener
	watchPlans []*watch.WatchPlan
}

var watches map[WatchType]watchData = make(map[WatchType]watchData)

type Listener interface {
	NotifyNodeUpdate(NotifyUpdateType, string)
	NotifyKeyUpdate(NotifyUpdateType, string, []byte)
	NotifyStoreUpdate(NotifyUpdateType, string, map[string][]byte)
}

func contains(wType WatchType, key string, elem interface{}) bool {
	ws, ok := watches[wType]
	if !ok {
		return false
	}

	list, ok := ws.listeners[key]

	if !ok {
		return false
	}

	v := reflect.ValueOf(list)

	for i := 0; i < v.Len(); i++ {
		if v.Index(i).Interface() == elem {
			return true
		}

	}
	return false
}

type watchconsul bool

func addListener(wtype WatchType, key string, listener Listener) watchconsul {
	var wc watchconsul = false
	if !contains(WATCH_TYPE_NODE, key, listener) {
		ws, ok := watches[wtype]
		if !ok {
			watches[wtype] = watchData{make(map[string][]Listener), make([]*watch.WatchPlan, 0)}
			ws = watches[wtype]
		}

		listeners, ok := ws.listeners[key]
		if !ok {
			listeners = make([]Listener, 0)
			wc = true
		}
		ws.listeners[key] = append(listeners, listener)
	}
	return wc
}

func getListeners(wtype WatchType, key string) []Listener {
	ws, ok := watches[wtype]
	if !ok {
		return nil
	}

	list, ok := ws.listeners[key]
	if ok {
		return list
	}
	return nil
}

func addWatchPlan(wtype WatchType, wp *watch.WatchPlan) {
	ws, ok := watches[wtype]
	if !ok {
		return
	}

	ws.watchPlans = append(ws.watchPlans, wp)
	watches[wtype] = ws
}

func stopWatches() {
	for _, ws := range watches {
		for _, wp := range ws.watchPlans {
			wp.Stop()
		}
		ws.watchPlans = ws.watchPlans[:0]
	}
}

func register(wtype WatchType, params map[string]interface{}, handler watch.HandlerFunc) {
	// Create the watch
	wp, err := watch.Parse(params)
	if err != nil {
		fmt.Printf("Register error : %s", err)
		return
	}
	addWatchPlan(wtype, wp)
	wp.Handler = handler
	cmdFlags := flag.NewFlagSet("watch", flag.ContinueOnError)
	httpAddr := command.HTTPAddrFlag(cmdFlags)
	// Run the watch
	if err := wp.Run(*httpAddr); err != nil {
		fmt.Printf("Error querying Consul agent: %s", err)
	}
}

var nodeCache []*api.Node

func compare(X, Y []*api.Node) []*api.Node {
	m := make(map[string]bool)

	for _, y := range Y {
		m[y.Address] = true
	}

	var ret []*api.Node
	for _, x := range X {
		if m[x.Address] {
			continue
		}
		ret = append(ret, x)
	}

	return ret
}

func updateNodeListeners(clusterNodes []*api.Node) {
	toDelete := compare(nodeCache, clusterNodes)
	toAdd := compare(clusterNodes, nodeCache)
	nodeCache = clusterNodes
	listeners := getListeners(WATCH_TYPE_NODE, "")
	if listeners == nil {
		return
	}
	for _, deleteNode := range toDelete {
		for _, listener := range listeners {
			listener.NotifyNodeUpdate(NOTIFY_UPDATE_DELETE, deleteNode.Address)
		}
	}

	for _, addNode := range toAdd {
		for _, listener := range listeners {
			listener.NotifyNodeUpdate(NOTIFY_UPDATE_ADD, addNode.Address)
		}
	}
}

func updateKeyListeners(idx uint64, key string, data interface{}) {
	listeners := getListeners(WATCH_TYPE_KEY, key)
	if listeners == nil {
		return
	}

	var kv *api.KVPair = nil
	var val []byte = nil
	updateType := NOTIFY_UPDATE_MODIFY

	if data != nil {
		kv = data.(*api.KVPair)
	}

	if kv == nil {
		updateType = NOTIFY_UPDATE_DELETE
	} else {
		updateType = NOTIFY_UPDATE_MODIFY
		if idx == kv.CreateIndex {
			updateType = NOTIFY_UPDATE_ADD
		}
		val = kv.Value
	}

	for _, listener := range listeners {
		listener.NotifyKeyUpdate(NotifyUpdateType(updateType), key, val)
	}
}

func registerForNodeUpdates() {
	// Compile the watch parameters
	params := make(map[string]interface{})
	params["type"] = "nodes"
	handler := func(idx uint64, data interface{}) {
		updateNodeListeners(data.([]*api.Node))
	}
	register(WATCH_TYPE_NODE, params, handler)
}

func RegisterForNodeUpdates(listener Listener) {
	wc := addListener(WATCH_TYPE_NODE, "", listener)
	if wc {
		registerForNodeUpdates()
	}
}

func registerForKeyUpdates(absKey string) {
	params := make(map[string]interface{})
	params["type"] = "key"
	params["key"] = absKey
	handler := func(idx uint64, data interface{}) {
		updateKeyListeners(idx, absKey, data)
	}
	register(WATCH_TYPE_KEY, params, handler)
}

func RegisterForKeyUpdates(store string, key string, listener Listener) {
	absKey := store + "/" + key
	wc := addListener(WATCH_TYPE_KEY, absKey, listener)
	if wc {
		registerForKeyUpdates(absKey)
	}
}

func registerForStoreUpdates(store string) {
	// Compile the watch parameters
	params := make(map[string]interface{})
	params["type"] = "keyprefix"
	params["prefix"] = store + "/"
	handler := func(idx uint64, data interface{}) {
		fmt.Println("NOT IMPLEMENTED Store Update :", idx, data)
	}
	register(WATCH_TYPE_STORE, params, handler)
}

func RegisterForStoreUpdates(store string, listener Listener) {
	wc := addListener(WATCH_TYPE_STORE, store, listener)
	if wc {
		registerForStoreUpdates(store)
	}
}

func watchForExistingRegisteredUpdates() {
	for wType, ws := range watches {
		glog.Infof("watchForExistingRegisteredUpdates : ", wType)
		for key, _ := range ws.listeners {
			glog.Infof("key : ", key)
			switch wType {
			case WATCH_TYPE_NODE:
				go registerForNodeUpdates()
			case WATCH_TYPE_KEY:
				go registerForKeyUpdates(key)
			case WATCH_TYPE_STORE:
				go registerForStoreUpdates(key)
			}
		}
	}
}
