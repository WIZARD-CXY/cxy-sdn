package server

import (
	"encoding/json"
	"github.com/WIZARD-CXY/cxy-sdn/util"
	_ "github.com/golang/glog"
	"github.com/gorilla/mux"
	"net/http"
)

type Err struct {
	code    int
	message string
}

type HttpApiFunc func(d *Daemon, w http.ResponseWriter, r *http.Request) *Err

type Handler struct {
	*Daemon
	fct HttpApiFunc
}

func (handler Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	ContainerPID  string `json:"container_pid"`
	Network       string `json:"network"`
	OvsPortID     string `json:"ovsport_id"`
}

func ServeAPI(d *Daemon) {
	r := createRouter(d)
	server := &http.Server{
		Addr:    "127.0.0.1:6675",
		Handler: r,
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
		},
	}

	for method, routes := range m {
		for uri, Func := range routes {
			handler := Handler{d, Func}
			r.Path(uri).Methods(method).Handler(handler)
		}
	}
	return r
}

// return the cxy-sdn version
func getVersion(d *Daemon, w http.ResponseWriter, r *http.Request) *Err {
	w.Write([]byte(util.VERSION))

	return nil
}

// get the bridge conf
func getConf(d *Daemon, w http.ResponseWriter, r *http.Request) *Err {
	conf, _ := json.Marshal(d.bridgeConf)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(conf)

	return nil
}

// set the bridge conf
func setConf(d *Daemon, w http.ResponseWriter, r *http.Request) *Err {
	if r.Body == nil {
		return &Err{http.StatusBadRequest, "SetConf requese has no body"}
	}

	cfg := &BridgeConf{}

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(cfg)

	if err != nil {
		return &Err{http.StatusInternalServerError, "setConf json decode failed"}
	}

	d.bridgeConf = cfg
	return nil
}

// get all the connections
func getCons(d *Daemon, w http.ResponseWriter, r *http.Request) *Err {
	data, _ := json.Marshal(d.connections)
	w.Header().Set("Content-type", "application/json; charset=utf-8")
	w.Write(data)

	return nil
}

// get all the existing network
func getNets(d *Daemon, w http.ResponseWriter, r *http.Request) *Err {
	networks, err := GetNetworks()
	if err != nil {
		return &Err{http.StatusInternalServerError, err.Error()}
	}

	data, err := json.Marshal(networks)

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)
	return nil

}
