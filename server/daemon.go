package server

import (
	/*"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"*/

	"github.com/codegangsta/cli"
	"github.com/golang/glog"
)

type Daemon struct {
	Conf *BridgeConf
}

func NewDaemon() *Daemon {
	return &Daemon{
		&BridgeConf{},
	}
}
func (d *Daemon) Run(ctx *cli.Context) {
	glog.Info("Daemon start")
	select {}

}
