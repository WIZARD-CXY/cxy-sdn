package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/WIZARD-CXY/cxy-sdn/agent"
	"github.com/WIZARD-CXY/cxy-sdn/util"
	"math"
	"net"
	"time"
)

const networkStore = "networkStore"
const vlanStore = "vlanStore"
const ipStore = "ipStore"
const defaultNetwork = "cxy"

var gatewayAddrs = []string{
	// Here we don't follow the convention of using the 1st IP of the range for the gateway.
	// This is to use the same gateway IPs as the /24 ranges, which predate the /16 ranges.
	// In theory this shouldn't matter - in practice there's bound to be a few scripts relying
	// on the internal addressing or other stupid things like that.
	// They shouldn't, but hey, let's not break them unless we really have to.
	"10.1.42.1/16",
	"10.42.42.1/16",
	"172.16.42.1/24",
	"172.16.43.1/24",
	"172.16.44.1/24",
	"10.0.42.1/24",
	"10.0.43.1/24",
	"172.17.42.1/16", // Don't use 172.16.0.0/16, it conflicts with EC2 DNS 172.16.0.23
	"10.0.42.1/16",   // Don't even try using the entire /8, that's too intrusive
	"192.168.42.1/24",
	"192.168.43.1/24",
	"192.168.44.1/24",
}

const vlanCount = 4096

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
	networkBytes, _, _ := netAgent.GetAll(networkStore)
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
		//already exist
		fmt.Printf("Network %s already exist in store\n", name)
		return network, errors.New("Network already exist")
	}

	// get the smallest unused vlan id from data store
	vlanID, err := allocateVlan()

	if err != nil {
		return nil, err
	}

	var gateway net.IP

	addr, err := util.GetIfaceAddr(name)

	if err != nil {
		fmt.Printf("Interface with name %s does not exist, Creating it\n", name)

		gateway = RequestIP(fmt.Sprint(vlanID), *subnet)
		network = &Network{name, subnet.String(), gateway.String(), vlanID}

		if err = AddInternalPort(ovsClient, bridgeName, name, vlanID); err != nil {
			return network, err
		}
		time.Sleep(1 * time.Second)

		gatewayCIDR := &net.IPNet{gateway, subnet.Mask}

		if err = util.SetMtu(name, mtu); err != nil {
			return network, err
		}

		if err = util.SetInterfaceIp(name, gatewayCIDR.String()); err != nil {
			return network, err
		}

		if err = util.InterfaceUp(name); err != nil {
			return network, err
		}

	} else {
		fmt.Printf("Interface %s already exists\n", name)
		ifaceAddr := addr.String()

		gateway, subnet, err = net.ParseCIDR(ifaceAddr)

		if err != nil {
			return nil, err
		}
		network = &Network{name, subnet.String(), gateway.String(), vlanID}
	}

	netBytes, _ := json.Marshal(network)

	if err != nil {
		return nil, err
	}

	err2 := netAgent.Put(networkStore, name, netBytes, nil)

	if err2 == netAgent.OUTDATED {
		releaseVlan(vlanID)
		ReleaseIP(gateway, *subnet, fmt.Sprint(vlanID))
		return CreateNetwork(name, subnet)
	}

	if err = setupIPTables(network.Name, network.Subnet); err != nil {
		return network, err
	}

	return network, nil

}

// this function is used to create network from network datastore
// assume the network whose name is `name` is already exist but have no interface
func CreateNetwork2(name string, subnet *net.IPNet) (*Network, error) {
	network, err := GetNetwork(name)

	if err != nil {
		//not already exist
		fmt.Printf("can't get %s in store, maybe communication err\n", name)
		return network, errors.New("Network not exist")
	}

	// get the smallest unused vlan id from data store
	vlanID := network.VlanID

	gateway := network.Gateway

	addr, err := util.GetIfaceAddr(name)

	if err != nil {
		fmt.Printf("Interface with name %s does not exist, Creating it\n", name)

		if err = AddInternalPort(ovsClient, bridgeName, name, vlanID); err != nil {
			return network, err
		}
		time.Sleep(1 * time.Second)

		gatewayCIDR := &net.IPNet{net.ParseIP(gateway), subnet.Mask}

		if err = util.SetMtu(name, mtu); err != nil {
			return network, err
		}

		if err = util.SetInterfaceIp(name, gatewayCIDR.String()); err != nil {
			return network, err
		}

		if err = util.InterfaceUp(name); err != nil {
			return network, err
		}

	} else {
		fmt.Printf("Interface %s already exists\n", name)
		ifaceAddr := addr.String()

		_, subnet, err = net.ParseCIDR(ifaceAddr)

		if err != nil {
			return nil, err
		}
		network = &Network{name, subnet.String(), gateway, vlanID}
	}

	if err = setupIPTables(network.Name, network.Subnet); err != nil {
		return network, err
	}

	return network, nil

}

func CreateDefaultNetwork(isBootstrap bool) (*Network, error) {
	subnet, err := GetAvailableSubnet()

	if err != nil {
		return &Network{}, err
	}
	if isBootstrap {
		return CreateNetwork(defaultNetwork, subnet)
	} else {
		return CreateNetwork2(defaultNetwork, subnet)
	}
}

func DeleteNetwork(name string) error {
	network, err := GetNetwork(name)
	if err != nil {
		return err
	}

	eccerror := netAgent.Delete(networkStore, name)
	if eccerror != netAgent.OK {
		return errors.New("Error deleting network")
	}
	releaseVlan(network.VlanID)

	if ovsClient == nil {
		return errors.New("OVS not connected")
	}
	deletePort(ovsClient, bridgeName, name)
	return nil
}

// used for minion node to sync the network from network Store
// ignore errors
func syncNetwork(d *Daemon) {
	//sync every 5 seconds
	for {
		networks, err := GetNetworks()
		if err != nil {
			fmt.Println("Error in getNetworks")
			time.Sleep(3 * time.Second)
			continue
		}

		// add interface
		for _, network := range networks {
			_, err := util.GetIfaceAddr(network.Name)

			if err != nil {
				// network not exsit create the interface from net store
				if err = AddInternalPort(ovsClient, bridgeName, network.Name, network.VlanID); err != nil {
					fmt.Println("add internal port err in syncNetwork", network.Name)
					continue
				}
				time.Sleep(1 * time.Second)

				if err = util.SetMtu(network.Name, mtu); err != nil {
					fmt.Println("set mtu err in syncNetwork", network.Name)
					continue
				}

				_, subnet, _ := net.ParseCIDR(network.Subnet)
				gatewayCIDR := &net.IPNet{net.ParseIP(network.Gateway), subnet.Mask}
				if err = util.SetInterfaceIp(network.Name, gatewayCIDR.String()); err != nil {
					fmt.Println("set ip err in syncNetwork", network.Name)
					continue
				}

				if err = util.InterfaceUp(network.Name); err != nil {
					fmt.Println("interface up err in syncNetwork", network.Name)
					continue
				}
				d.Gateways[network.Name] = struct{}{}

				if err = setupIPTables(network.Name, network.Subnet); err != nil {
					fmt.Println("iptable setup err in syncNetwork", network.Name)
					continue
				}
				fmt.Println(network.Name + " network created")
			}
		}

		//delete unused interface
		var found bool
		for k, _ := range d.Gateways {
			found = false
			for _, network := range networks {
				if network.Name == k {
					//find network in the datastore
					found = true
					break
				}
			}

			// not found interface named k, delete it
			if !found {
				deletePort(ovsClient, bridgeName, k)
				delete(d.Gateways, k)
				fmt.Println("delete unused interface", k)
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func allocateVlan() (uint, error) {
	vlanBytes, _, ok := netAgent.Get(vlanStore, "vlan")

	// not put the key already
	if !ok {
		vlanBytes = make([]byte, vlanCount/8)
	}

	curVal := make([]byte, vlanCount/8)
	copy(curVal, vlanBytes)

	vlanID := util.TestAndSet(vlanBytes)

	if vlanID > vlanCount {
		return uint(vlanID), errors.New("All vlanID have been used")
	}

	netAgent.Put(vlanStore, "vlan", vlanBytes, curVal)

	return uint(vlanID), nil

}

func releaseVlan(vlanID uint) {
	oldVal, _, ok := netAgent.Get(vlanStore, "vlan")

	if !ok {
		oldVal = make([]byte, vlanCount/8)
	}
	curVal := make([]byte, vlanCount/8)
	copy(curVal, oldVal)

	util.Clear(curVal, vlanID-1)
	err := netAgent.Put(vlanStore, "vlan", curVal, oldVal)
	if err == netAgent.OUTDATED {
		releaseVlan(vlanID)
	}
}
func GetAvailableGwAddress(bridgeIP string) (gwaddr string, err error) {
	if len(bridgeIP) != 0 {
		_, _, err = net.ParseCIDR(bridgeIP)
		if err != nil {
			return
		}
		gwaddr = bridgeIP
	} else {
		for _, addr := range gatewayAddrs {
			_, dockerNetwork, err := net.ParseCIDR(addr)
			if err != nil {
				return "", err
			}
			if err = util.CheckRouteOverlaps(dockerNetwork); err != nil {
				continue
			}
			gwaddr = addr
			break
		}
	}
	if gwaddr == "" {
		return "", errors.New("No available gateway addresses")
	}
	return gwaddr, nil
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

// ipStore manage the cluster ip resource
// key is the vlan/subnet, value is the available ip address bytes

// Get an IP from the unused subnet and mark it as used
func RequestIP(vlanID string, subnet net.IPNet) net.IP {
	ipCount := util.IPCount(subnet)
	bc := int(ipCount / 8)
	partial := int(math.Mod(ipCount, float64(8)))

	if partial != 0 {
		bc += 1
	}

	oldArray, _, ok := netAgent.Get(ipStore, vlanID+"-"+subnet.String())

	if !ok {
		oldArray = make([]byte, bc)
	}

	newArray := make([]byte, len(oldArray))

	copy(newArray, oldArray)

	pos := util.TestAndSet(newArray)

	err := netAgent.Put(ipStore, vlanID+"-"+subnet.String(), newArray, oldArray)

	if err == netAgent.OUTDATED {
		return RequestIP(vlanID, subnet)
	}

	var num uint32

	buf := bytes.NewBuffer(subnet.IP)

	err2 := binary.Read(buf, binary.BigEndian, &num)

	if err2 != nil {
		fmt.Println(err)
		return nil
	}

	num += pos

	buf2 := new(bytes.Buffer)
	err2 = binary.Write(buf2, binary.BigEndian, num)

	if err2 != nil {
		return nil
	}
	return net.IP(buf2.Bytes())

}

//  Mark a specified ip as used, return trus as success
func MarkUsed(addr net.IP, subnet net.IPNet) bool {
	oldArray, _, ok := netAgent.Get(ipStore, subnet.String())

	if !ok {
		return false
	}

	newArray := make([]byte, len(oldArray))
	copy(newArray, oldArray)

	var num1, num2 uint32

	buf1 := bytes.NewBuffer(addr.To4())
	binary.Read(buf1, binary.BigEndian, &num1)

	buf := bytes.NewBuffer(subnet.IP)

	binary.Read(buf, binary.BigEndian, &num2)

	pos := uint32(num1 - num2 - 1)

	util.Set(newArray, pos)

	err2 := netAgent.Put(ipStore, subnet.String(), newArray, oldArray)

	if err2 == netAgent.OUTDATED {
		MarkUsed(addr, subnet)
	}

	return true

}

// Release the given IP from the subnet of vlan
func ReleaseIP(addr net.IP, subnet net.IPNet, vlanID string) bool {
	oldArray, _, ok := netAgent.Get(ipStore, vlanID+"-"+subnet.String())

	if !ok {
		return false
	}

	newArray := make([]byte, len(oldArray))
	copy(newArray, oldArray)

	var num1, num2 uint32

	buf1 := bytes.NewBuffer(addr.To4())
	binary.Read(buf1, binary.BigEndian, &num1)

	buf := bytes.NewBuffer(subnet.IP)

	binary.Read(buf, binary.BigEndian, &num2)

	pos := uint(num1 - num2 - 1)

	util.Clear(newArray, pos)

	err2 := netAgent.Put(ipStore, vlanID+"-"+subnet.String(), newArray, oldArray)

	if err2 == netAgent.OUTDATED {
		return ReleaseIP(addr, subnet, vlanID)
	}

	return true
}
