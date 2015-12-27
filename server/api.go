// package server implements the server logic of
// cxy-sdn software including a apiserver and consul
// datastore backend and ovs connection manager

package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"

	"github.com/gorilla/mux"
)

type HttpErr struct {
	code    int
	message string
}

const version = "10.0"

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
	ContainerID      string        `json:"containerID"`
	ContainerName    string        `json:"containerName"`
	ContainerPID     string        `json:"containerPID"`
	RequestIp        string        `json:"requestIP,omitempty"`
	Network          string        `json:"network"`
	OvsPortID        string        `json:"ovsPortID"`
	BandWidth        string        `json:"bandWidth,omitempty"`
	Delay            string        `json:"delay,omitempty"`
	RXTotal          uint64        `json:"rxKbytes"` // in KB
	TXTotal          uint64        `json:"txKbytes"` // in KB
	RXRate           float64       `json:"rxRate"`   // in Kb/s
	TXRate           float64       `json:"txRate"`   // in Kb/s
	ConnectionDetail OvsConnection `json:"ovs_connectionDetails"`
}

func ServeApi(d *Daemon) {
	server := &http.Server{
		Addr:    "127.0.0.1:8888",
		Handler: createRouter(d),
	}
	// start a pprof server
	go http.ListenAndServe("127.0.0.1:8889", nil)

	server.ListenAndServe()
}

func createRouter(d *Daemon) *mux.Router {
	r := mux.NewRouter()
	// configure the router to always run this handler when it couldn't match a request to any other handler
	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("%s not found\n", r.URL)))
	})

	m := map[string]map[string]HttpApiFunc{
		"GET": {
			"/version":            getVersion,
			"/configuration":      getConf,
			"/networks":           getNets,
			"/network/{name:.*}":  getNet,
			"/connections":        getConns,
			"/connection/{id:.*}": getConn,
		},
		"POST": {
			"/configuration": setConf,
			"/network":       createNet,
			"/cluster/join":  joinCluster,
			"/cluster/leave": leaveCluster,
			"/connection":    createConn,
			"/qos/{id:.*}":   createQos,
		},
		"PUT": {
			"/qos/{id:.*}": updateQos,
		},
		"DELETE": {
			"/network/{name:.*}":  delNet,
			"/connection/{id:.*}": delConn,
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
	w.Write([]byte(version))

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
		return &HttpErr{http.StatusBadRequest, "SetConf request has no body"}
	}

	cfg := &BridgeConf{}

	err := json.NewDecoder(r.Body).Decode(cfg)

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, "setConf json decode failed"}
	}

	d.bridgeConf = cfg
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

// get one specified network detail
func getNet(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	// get the network name
	vars := mux.Vars(r)
	name := vars["name"]

	network, err := GetNetwork(name)

	if err != nil {
		return &HttpErr{http.StatusNotFound, err.Error()}
	}

	data, err := json.Marshal(network)

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	w.Header().Set("Content-type", "application/json; charset=utf-8")
	w.Write(data)

	return nil
}

// create a network
func createNet(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	if r.Body == nil {
		return &HttpErr{http.StatusBadRequest, "request body is empty"}
	}

	network := &Network{}
	err := json.NewDecoder(r.Body).Decode(network)

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	_, cidr, err := net.ParseCIDR(network.Subnet)

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	newNet, err := CreateNetwork(network.Name, cidr)

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	data, _ := json.Marshal(newNet)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)

	return nil
}

// delete one specified network
func delNet(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	vars := mux.Vars(r)
	name := vars["name"]

	err := DeleteNetwork(name)

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	return nil
}

// node join the cluster
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

	d.clusterChan <- &NodeCtx{ip.String(), nodeJoin}
	return nil
}

// node leave the cluster
func leaveCluster(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	fmt.Println("Node leave cluster")
	d.clusterChan <- &NodeCtx{"", nodeLeave}

	return nil
}

// get all connections
func getConns(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	d.connections.RLock()
	data, err := json.Marshal(d.connections.rm)
	d.connections.RUnlock()

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)

	return nil
}

// get one specified connection
func getConn(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	vars := mux.Vars(r)

	containerId := vars["id"]
	con := d.connections.Get(containerId)

	if con == nil {
		return &HttpErr{http.StatusNotFound, containerId}
	}

	data, err := json.Marshal(con.(*Connection))

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)

	return nil
}

// create a container connection
func createConn(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	if r.Body == nil {
		return &HttpErr{http.StatusBadRequest, "request body is empty"}
	}

	con := &Connection{}
	err := json.NewDecoder(r.Body).Decode(con)

	if err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	if con.Network == "" {
		con.Network = defaultNetwork
	}

	ctx := &ConnectionCtx{
		addConn,
		con,
		make(chan *Connection),
	}

	d.connectionChan <- ctx

	res := <-ctx.Result

	if res.OvsPortID == "-1" {
		return &HttpErr{http.StatusBadRequest, "resp body not valid"}
	}

	data, _ := json.Marshal(res)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)

	return nil
}

// delete the ovs and container connection
func delConn(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	vars := mux.Vars(r)
	containerId := vars["id"]

	con := d.connections.Get(containerId)

	if con == nil {
		return &HttpErr{http.StatusNotFound, "container not found"}
	}

	ctx := &ConnectionCtx{
		deleteConn,
		con.(*Connection),
		make(chan *Connection),
	}

	d.connectionChan <- ctx
	<-ctx.Result

	return nil
}

// create a qos for a container
func createQos(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	bw := r.FormValue("bw")
	delay := r.FormValue("delay")

	vars := mux.Vars(r)
	containerId := vars["id"]

	if bw == "" && delay == "" {
		return &HttpErr{http.StatusBadRequest, "bw and delay is empty"}
	}

	ok := d.connections.Check(containerId)

	if !ok {
		return &HttpErr{http.StatusNotFound, "container not found"}
	}

	if err := addQos(d, containerId, bw, delay); err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	return nil
}

// update a qos for a container
func updateQos(d *Daemon, w http.ResponseWriter, r *http.Request) *HttpErr {
	bw := r.FormValue("bw")
	delay := r.FormValue("delay")

	vars := mux.Vars(r)
	containerId := vars["id"]

	if bw == "" && delay == "" {
		return &HttpErr{http.StatusBadRequest, "bw and delay is empty"}
	}

	ok := d.connections.Check(containerId)

	if !ok {
		return &HttpErr{http.StatusNotFound, "container not found"}
	}

	if err := changeQos(d, containerId, bw, delay); err != nil {
		return &HttpErr{http.StatusInternalServerError, err.Error()}
	}

	return nil
}
