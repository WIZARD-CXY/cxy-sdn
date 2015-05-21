package util

// sdn conf related function

type config struct {
	Daemon DaemonCfg
	// Add more Configs such as ClusterCfg, OvsCfg, etc.
}

type DaemonCfg struct {
	Bootstrap bool
	Debug     bool
}

var spConfig config
var Daemon DaemonCfg
