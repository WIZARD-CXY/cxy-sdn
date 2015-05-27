package server

import (
	"encoding/json"
	"errors"
	"github.com/WIZARD-CXY/cxy-sdn/util"
	"github.com/WIZARD-CXY/netAgent"
	"github.com/golang/glog"
	"net"
)

const networkStore = "network"
const vlanStore = "vlan"
const defaultNetwork = "default"

const vlanCount = 4096

// borrow from docker
var gatewayAddrs = []string{
	"10.1.42.1/16",
	"10.42.42.1/16",
	"172.16.42.1/24",
	"172.16.43.1/24",
	"172.16.44.1/24",
	"10.0.42.1/24",
	"10.0.43.1/24",
	"172.17.42.1/16",
	"10.0.42.1/16",
	"192.168.42.1/24",
	"192.168.43.1/24",
	"192.168.44.1/24",
}

type Network struct {
	Name    string `json:"name"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
	VlanID  uint   `json:"vlanid"`
}

// get the network detail of a given name
func GetNetwork(name string) (*Network, error) {
	netByte, _, ok := netAgent.Get(networkStore, name)
	if ok {
		network := &Network{}
		err := json.Unmarshal(netByte, network)

		if err != nil {
			return nil, err
		}

		return network, nil
	}
	return nil, errors.New("Network " + name + " not exist")
}

func GetNetworks() ([]Network, error) {
	networkBytes, _, ok := netAgent.GetAll(networkStore)
	networks := make([]Network, 0)

	for _, networkByte := range networkBytes {
		network := Network{}
		err := json.Unmarshal(networkByte, &network)

		if err != nil {
			return nil, err
		}

		networks = append(networks, network)
	}
	return networks, nil
}
func CreateNetwork(name string, subnet *net.IPNet) (*Network, error) {
	network, err := GetNetwork(name)

	if err == nil {
		glog.Infof("Network %s already exist", name)
		return network, nil
	}

	// get the smallest unused vlan id from data store
	vlanID, err := allocateVlan()

	if err != nil {
		glog.Infof("Vlan not available")
		return nil, err
	}

	var gateway net.IP

	addr, err := util.GetIfaceAddr(name)

	if err != nil {
		glog.Infof("Network interface %s not exist", name)

		if ovs == nil {
			return nil, errors.New("OVS not connected")
		}

		gateway = IPAMRequest(*subnet)
		network = &Network{name, subnet.String(), gateway.String(), vlanID}
	}

}

func CreateDefaultNetwork() (*Network, error) {
	CreateNetwork(defaultNetwork, subnet)
}

func allocateVlan() (uint, error) {
	vlanBytes, _, ok := netAgent.Get(vlanStore, "vlan")

	// not put the key already
	if !ok {
		vlanBytes := make([]byte, vlanCount/8)

	}

	curVal := make([]byte, vlanCount/8)
	copy(curVal, vlanBytes)

	vlanID := util.TestAndSet(vlanBytes)

	if vlanID > vlanCount {
		return vlanID, errors.New("All vlanID have been used")
	}

	err := netAgent.Put(vlanStore, "vlan", vlanBytes, curVal)

	return vlanID, nil

}

func GetAvailableSubnet() (subnet *net.IPNet, err error) {
	for _, addr := range gatewayAddrs {
		_, dockerNetwork, err := net.ParseCIDR(addr)
		if err != nil {
			return &net.IPNet{}, err
		}
		if err = util.CheckRouteOverlaps(dockerNetwork); err == nil {
			return dockerNetwork, nil
		}
	}

	return &net.IPNet{}, errors.New("No available GW address")
}
