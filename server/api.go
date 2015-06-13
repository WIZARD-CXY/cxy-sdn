package server

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net"
	"net/http"
	"net/url"
)

type HttpErr struct {
	code    int
	message string
}

const VERSION = "0.1"

type HttpApiFunc func(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr

// myHandler implement http.Handler
type myHandler struct {
	*Daemon
	fct HttpApiFunc
}

func (handler myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := handler.fct(handler.Daemon, w, r)
	if err != nil {
		http.Error(w, err.message, err.code)
	}
}

type BridgeConf struct {
	BridgeIP   string `json:"bridgeIP"`
	BridgeName string `json:"bridgeName"`
	BridgeCIDR string `json:"bridgeCIDR"`
	BridgeMTU  int    `json:"bridgeMTU"`
}

type Connection struct {
	ContainerID   string `json:"containerID"`
	ContainerName string `json:"containerName"`
	ContainerPID  string `json:"containerPID"`
	Network       string `json:"networkName"`
	OvsPortID     string `json:"ovsPortID"`
}

func ServeApi(d *Daemon) {
	server := &http.Server{
		Addr:    "127.0.0.1:6675",
		Handler: createRouter(d),
	}
	server.ListenAndServe()
}

func createRouter(d *Daemon) *mux.Router {
	r := mux.NewRouter()
	m := map[string]map[string]HttpApiFunc{
		"GET": {
			"/version":       getVersion,
			"/configuration": getConf,
			"/networks":      getNets,
		},
		"POST": {
			"/configuration": setConf,
			"/cluster/join":  joinCluster,
			"/cluster/leave": leaveCluster,
		},
	}

	for method, routes := range m {
		for uri, Func := range routes {
			handler := myHandler{d, Func}
			r.Path(uri).Methods(method).Handler(handler)
		}
	}
	return r
}

// return the cxy-sdn version
func getVersion(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	w.Write([]byte(VERSION))

	return nil
}

// get the ovs bridge conf
func getConf(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	conf, _ := json.Marshal(d.bridgeConf)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(conf)

	return nil
}

// set the bridge conf
func setConf(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	if r.Body == nil {
		return &HttpErr{http.StatusBadRequest, "SetConf requese has no body"}
	}

	cfg := &BridgeConf{}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(cfg)

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, "setConf json decode failed"}
	}

	d.bridgeConf = cfg
	return nil
}

// get all the connections
func getCons(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	data, _ := json.Marshal(d.connections)
	w.Header().Set("Content-type", "application/json; charset=utf-8")
	w.Write(data)

	return nil
}

// get all the existing network
func getNets(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	networks, err := GetNetworks()
	if err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	data, err := json.Marshal(networks)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)
	return nil
}

func joinCluster(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	if r.URL.RawQuery == "" {
		return &HttpErr{http.StatusBadRequest, "address missing"}
	}

	kvs, err := url.ParseQuery(r.URL.RawQuery)

	if err != nil {
		return &HttpErr{http.StatusBadRequest, "parse query string error"}
	}

	addr, ok := kvs["address"]

	if !ok || addr[0] == "" {
		return &HttpErr{http.StatusBadRequest, "address parameter not exist"}
	}

	fmt.Println("Join to cluster", addr[0])

	ip := net.ParseIP(addr[0])
	if ip == nil {
		return &HttpErr{http.StatusBadRequest, "Invalid IP address"}
	}

	d.clusterChan <- &ClusterInfo{ip.String(), nodeJoin}
	return nil
}

func leaveCluster(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	d.clusterChan <- &ClusterInfo{"", nodeLeave}

	return nil
}
