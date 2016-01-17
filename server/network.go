package server

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"github.com/WIZARD-CXY/cxy-sdn/netAgent"
	"github.com/WIZARD-CXY/cxy-sdn/util"
)

const networkStore = "networkStore"
const vlanStore = "vlanStore"
const ipStore = "ipStore"
const defaultNetwork = "cxy"

var gatewayAddrs = []string{
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

const vlanCount = 1000000

type Network struct {
	Name    string `json:"name"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
	VNI     uint   `json:"vni"`
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

func CreateDefaultNetwork() (*Network, error) {
	subnet, err := GetAvailableSubnet()

	if err != nil {
		return &Network{}, err
	}
	return CreateNetwork(defaultNetwork, subnet)
}

func CreateNetwork(name string, subnet *net.IPNet) (*Network, error) {
	network, err := GetNetwork(name)

	if err == nil {
		//already exist
		log.Printf("Network %s already exist in store\n", name)
		return network, errors.New("Network already exist")
	}

	// get the smallest unused vlan id from data store
	VNI, err := allocateVNI()

	if err != nil {
		return nil, err
	}

	var gateway net.IP

	addr, err := util.GetIfaceAddr(name)

	if err != nil {
		log.Printf("Interface with name %s does not exist, Creating it\n", name)

		gateway = RequestIP(fmt.Sprint(VNI), *subnet)

		network = &Network{name, subnet.String(), gateway.String(), VNI}

		if err = AddInternalPort(ovsClient, bridgeName, name, VNI); err != nil {
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
		ifaceAddr := addr.String()
		// even though the interface exist, mark its IP as used by using RequestIP to
		// let the ipam and I happy
		RequestIP(fmt.Sprint(VNI), *subnet)

		log.Printf("Interface %s already exists with IP %s, subnet %s, in %d vlan\n", name, ifaceAddr, subnet.String(), VNI)

		gateway, subnet, err = net.ParseCIDR(ifaceAddr)

		if err != nil {
			return nil, err
		}
		network = &Network{name, subnet.String(), gateway.String(), VNI}
	}

	netBytes, _ := json.Marshal(network)

	if err != nil {
		return nil, err
	}

	err2 := netAgent.Put(networkStore, name, netBytes, nil)

	if err2 == netAgent.OUTDATED {
		releaseVNI(VNI)
		ReleaseIP(gateway, *subnet, fmt.Sprint(VNI))
		return CreateNetwork(name, subnet)
	}

	if err = setupIPTables(network.Name, network.Subnet); err != nil {
		return network, err
	}

	return network, nil

}

// this function is used to create network from network datastore
// assume the network whose name is `name` is already exist but have no interface on the node
/*func CreateNetwork2(name string, subnet *net.IPNet) (*Network, error) {
	network, err := GetNetwork(name)

	if err != nil {
		//not already exist
		log.Printf("can't get %s in store, maybe communication err\n", name)
		return network, errors.New("Network not exist")
	}

	gateway := network.Gateway

	addr, err := util.GetIfaceAddr(name)

	if err != nil {
		log.Printf("Interface with name %s does not exist, Creating it\n", name)

		if err = AddInternalPort(ovsClient, bridgeName, name, network.VNI); err != nil {
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
		log.Printf("Interface %s already exists\n", name)
		ifaceAddr := addr.String()

		_, subnet, err = net.ParseCIDR(ifaceAddr)

		if err != nil {
			return nil, err
		}
		network = &Network{name, subnet.String(), gateway, network.VNI}
	}

	if err = setupIPTables(network.Name, network.Subnet); err != nil {
		return network, err
	}

	return network, nil

}*/

func DeleteNetwork(name string) error {
	network, err := GetNetwork(name)
	if err != nil {
		return err
	}

	errcode := netAgent.Delete(networkStore, name)
	if errcode != netAgent.OK {
		return errors.New("Error deleting network")
	}
	releaseVNI(network.VNI)

	if ovsClient == nil {
		return errors.New("OVS not connected")
	}
	deletePort(ovsClient, bridgeName, name)
	return nil
}

// used for client node to sync the network from network Store
// ignore errors
func syncNetwork(d *Daemon) {
	//sync every 5 seconds
	for {
		networks, err := GetNetworks()
		if err != nil {
			log.Println("Error in getNetworks")
			time.Sleep(3 * time.Second)
			continue
		}

		// add interface
		for _, network := range networks {
			_, err := util.GetIfaceAddr(network.Name)

			if err != nil {
				// network not exsit create the interface from net store
				if err = AddInternalPort(ovsClient, bridgeName, network.Name, network.VNI); err != nil {
					log.Println("add internal port err in syncNetwork", network.Name)
					continue
				}
				time.Sleep(1 * time.Second)

				if err = util.SetMtu(network.Name, mtu); err != nil {
					log.Println("set mtu err in syncNetwork", network.Name)
					continue
				}

				_, subnet, _ := net.ParseCIDR(network.Subnet)
				gatewayCIDR := &net.IPNet{net.ParseIP(network.Gateway), subnet.Mask}
				if err = util.SetInterfaceIp(network.Name, gatewayCIDR.String()); err != nil {
					log.Println("set ip err in syncNetwork", network.Name)
					continue
				}

				if err = util.InterfaceUp(network.Name); err != nil {
					log.Println("interface up err in syncNetwork", network.Name)
					continue
				}
				d.Gateways[network.Name] = struct{}{}

				if err = setupIPTables(network.Name, network.Subnet); err != nil {
					log.Println("iptable setup err in syncNetwork", network.Name)
					continue
				}
				log.Println(network.Name + " network created")
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
				log.Println("delete unused interface", k)
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func allocateVNI() (uint, error) {
	vlanBytes, _, ok := netAgent.Get(vlanStore, "vlan")

	// not put the key already
	if !ok {
		vlanBytes = make([]byte, vlanCount/8)
	}

	curVal := make([]byte, vlanCount/8)
	copy(curVal, vlanBytes)

	VNI := util.TestAndSet(vlanBytes)

	if VNI > vlanCount {
		return uint(VNI), errors.New("All VNI have been used")
	}

	netAgent.Put(vlanStore, "vlan", vlanBytes, curVal)

	return uint(VNI), nil

}

func releaseVNI(VNI uint) {
	oldVal, _, ok := netAgent.Get(vlanStore, "vlan")

	if !ok {
		oldVal = make([]byte, vlanCount/8)
	}
	curVal := make([]byte, vlanCount/8)
	copy(curVal, oldVal)

	util.Clear(curVal, VNI-1)
	err := netAgent.Put(vlanStore, "vlan", curVal, oldVal)
	if err == netAgent.OUTDATED {
		releaseVNI(VNI)
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
func RequestIP(VNI string, subnet net.IPNet) net.IP {
	ipCount := util.IPCount(subnet)
	bc := int(ipCount / 8)
	partial := int(math.Mod(ipCount, float64(8)))

	if partial != 0 {
		bc += 1
	}

	oldArray, _, ok := netAgent.Get(ipStore, VNI+"-"+subnet.String())

	if !ok {
		oldArray = make([]byte, bc)
	}

	newArray := make([]byte, len(oldArray))

	copy(newArray, oldArray)

	pos := util.TestAndSet(newArray)

	err := netAgent.Put(ipStore, VNI+"-"+subnet.String(), newArray, oldArray)

	if err == netAgent.OUTDATED {
		return RequestIP(VNI, subnet)
	}

	var num uint32

	buf := bytes.NewBuffer(subnet.IP)

	err2 := binary.Read(buf, binary.BigEndian, &num)

	if err2 != nil {
		log.Println(err)
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

// Mark a specified ip as used, return true as success
func MarkUsed(VNI string, addr net.IP, subnet net.IPNet) bool {
	oldArray, _, ok := netAgent.Get(ipStore, VNI+"-"+subnet.String())

	if !ok {
		// the kv pair not exist yet
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

	err2 := netAgent.Put(ipStore, VNI+"-"+subnet.String(), newArray, oldArray)

	if err2 == netAgent.OUTDATED {
		MarkUsed(VNI, addr, subnet)
	}

	return true

}

// Release the given IP from the subnet of vlan
func ReleaseIP(addr net.IP, subnet net.IPNet, VNI string) bool {
	oldArray, _, ok := netAgent.Get(ipStore, VNI+"-"+subnet.String())

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

	err2 := netAgent.Put(ipStore, VNI+"-"+subnet.String(), newArray, oldArray)

	if err2 == netAgent.OUTDATED {
		return ReleaseIP(addr, subnet, VNI)
	}

	return true
}
