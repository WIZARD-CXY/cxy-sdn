package server

import (
	"github.com/WIZARD-CXY/cxy-sdn/util"
	_ "github.com/golang/glog"
	"github.com/gorilla/mux"
	"net/http"
)

type srvErr struct {
	code    int
	message string
}

type HttpApiFunc func(d *Daemon, w http.ResponseWriter, r *http.Request) *srvErr

type appHandler struct {
	*Daemon
	h HttpApiFunc
}

func (ah appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := ah.h(ah.Daemon, w, r)
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
			"/version": getVersion,
		},
	}

	for method, routes := range m {
		for uri, Func := range routes {
			handler := appHandler{d, Func}
			r.Path(uri).Methods(method).Handler(handler)
		}
	}
	return r
}

func getVersion(d *Daemon, w http.ResponseWriter, r *http.Request) *srvErr {
	w.Write([]byte(util.VERSION))

	return nil
}
