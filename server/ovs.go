package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/WIZARD-CXY/cxy-sdn/util"
	// "github.com/golang/glog"
	"github.com/socketplane/libovsdb"
	"github.com/vishvananda/netns"
)

const mtu = 1440
const bridgeName = "ovs-br0"

var ovsClient *libovsdb.OvsdbClient
var ContextCache map[string]string

type OvsConnection struct {
	Name    string `json:"name"`
	Ip      string `json:"ip"`
	Subnet  string `json:"subnet"`
	Mac     string `json:"mac"`
	Gateway string `json:"gateway"`
}

const (
	addConn = iota
	deleteConn
)

type ConnectionCtx struct {
	Action     int
	Connection *Connection
	Result     chan *Connection
}

func init() {
	var err error
	ovsClient, err = ovs_connect()
	if err != nil {
		fmt.Println("Error connecting OVS ", err)
	} else {
		ovsClient.Register(notifier{})
	}
	ContextCache = make(map[string]string)
	populateContextCache()
}

func CreateBridge() (string, error) {
	var bridgeUUID string
	if ovsClient == nil {
		return "", errors.New("OVS not connected")
	}
	// If the bridge has been created, a internal port with the same name should exist
	exists, err := portExists(ovsClient, bridgeName)
	if err != nil {
		return "", err
	}
	if !exists {
		bridgeUUID, err = CreateOVSBridge(ovsClient, bridgeName)
		if err != nil {
			return "", err
		}
		exists, err = portExists(ovsClient, bridgeName)
		if err != nil {
			return "", err
		}
		if !exists {
			return "", errors.New("Error creating Bridge")
		}
	}
	return bridgeUUID, nil
}

func DeleteBridge(bridgeUUID string) error {
	if ovsClient == nil {
		return errors.New("OVS not connected")
	}
	if err := DeleteOVSBridge(ovsClient, bridgeName, bridgeUUID); err != nil {
		return err
	}
	return nil
}

func AddPeer(peerIp string) error {
	if ovsClient == nil {
		return errors.New("OVS not connected")
	}
	addVxlanPort(ovsClient, bridgeName, "vxlan-"+peerIp, peerIp)
	return nil
}

func DeletePeer(peerIp string) error {
	if ovsClient == nil {
		return errors.New("OVS not connected")
	}
	deletePort(ovsClient, bridgeName, "vxlan-"+peerIp)
	return nil
}

func connHandler(d *Daemon) {
	for {
		c := <-d.connectionChan
		fmt.Println("receive new conn")

		switch c.Action {
		case addConn:
			connDetail, err := addConnection(c.Connection.ContainerPID, c.Connection.Network, c.Connection.RequestIp)
			if err != nil {
				fmt.Printf("err is %+v\n", err)
				c.Connection.OvsPortID = "-1"
				c.Result <- c.Connection
				continue
			}
			fmt.Printf("connDetails %v\n", connDetail)
			c.Connection.OvsPortID = connDetail.Name
			c.Connection.ConnectionDetail = connDetail
			d.connections[c.Connection.ContainerID] = c.Connection
			c.Result <- c.Connection
		case deleteConn:
			DeleteConnection(c.Connection.ConnectionDetail, c.Connection.Network)
			delete(d.connections, c.Connection.ContainerID)
			c.Result <- c.Connection
		}
	}
}

func addConnection(nspid, networkName, requestIp string) (ovsConnection OvsConnection, err error) {
	var (
		bridge = bridgeName
		prefix = "ovs"
	)
	ovsConnection = OvsConnection{}
	err = nil

	if bridge == "" {
		err = fmt.Errorf("bridge is not available")
		return
	}

	if networkName == "" {
		networkName = defaultNetwork
	}
	fmt.Println("haha network name", networkName, nspid)

	bridgeNetwork, err := GetNetwork(networkName)
	if err != nil {
		return ovsConnection, err
	}

	portName, err := createOvsInternalPort(prefix, bridge, bridgeNetwork.VlanID)
	if err != nil {
		return
	}
	// Add a dummy sleep to make sure the interface is seen by the subsequent calls.
	time.Sleep(time.Second * 1)
	fmt.Println("newportName is", portName)

	_, subnet, _ := net.ParseCIDR(bridgeNetwork.Subnet)

	var ip net.IP
	if requestIp == "" {
		// if not request a static ip, using system auto-choose
		ip = RequestIP(fmt.Sprint(bridgeNetwork.VlanID), *subnet)
	} else {
		// if request ip, mark it as used and use it
		ip = net.ParseIP(requestIp)
		MarkUsed(ip, *subnet)
	}

	fmt.Println("newIP is", ip)
	mac := generateMacAddr(ip).String()

	subnetString := subnet.String()
	subnetPrefix := subnetString[len(subnetString)-3 : len(subnetString)]

	ovsConnection = OvsConnection{portName, ip.String(), subnetPrefix, mac, bridgeNetwork.Gateway}

	if err = os.Symlink(filepath.Join(os.Getenv("PROCFS"), nspid, "ns/net"),
		filepath.Join("/var/run/netns", nspid)); err != nil {
		return
	}

	// Lock the OS Thread so we don't accidentally switch namespaces
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// save the old netns
	origns, err := netns.Get()
	if err != nil {
		return
	}

	defer origns.Close()

	targetns, err := netns.GetFromName(nspid)
	if err != nil {
		return
	}

	defer targetns.Close()

	// set port in targetns
	if err = util.SetInterfaceInNamespaceFd(portName, uintptr(int(targetns))); err != nil {
		fmt.Println("Set port in ns err in addcon")
		return
	}

	// switch to targetns
	if err = netns.Set(targetns); err != nil {
		return
	}

	// In the end switch back to the original namespace
	defer netns.Set(origns)

	if err = util.SetMtu(portName, mtu); err != nil {
		fmt.Println("set mtu error in addCon")
		return
	}

	if err = util.SetInterfaceIp(portName, ip.String()+subnetPrefix); err != nil {
		fmt.Println("SetInterfaceip error in addcon")
		return
	}

	if err = util.SetInterfaceMac(portName, mac); err != nil {
		fmt.Println("SetInterfacemac error in addcon")
		return
	}

	if err = util.InterfaceUp(portName); err != nil {
		fmt.Println("Interfaceup error in addcon")
		return
	}

	if err = util.SetDefaultGateway(bridgeNetwork.Gateway, portName); err != nil {
		fmt.Println("SetdefaultGateway error in addcon")
		return
	}

	return ovsConnection, nil
}

func UpdateConnectionContext(ovsPort string, key string, context string) error {
	return UpdatePortContext(ovsClient, ovsPort, key, context)
}

func populateContextCache() {
	if ovsClient == nil {
		return
	}
	tableCache := GetTableCache("Interface")
	for _, row := range tableCache {
		config, ok := row.Fields["other_config"]
		ovsMap := config.(libovsdb.OvsMap)
		other_config := map[interface{}]interface{}(ovsMap.GoMap)
		if ok {
			container_id, ok := other_config[CONTEXT_KEY]
			if ok {
				ContextCache[container_id.(string)] = other_config[CONTEXT_VALUE].(string)
			}
		}
	}
}

func DeleteConnection(connection OvsConnection, networkName string) error {
	if ovsClient == nil {
		return errors.New("OVS not connected")
	}
	deletePort(ovsClient, bridgeName, connection.Name)
	ip := net.ParseIP(connection.Ip)
	_, subnet, _ := net.ParseCIDR(connection.Ip + connection.Subnet)

	bridgeNetwork, err := GetNetwork(networkName)
	if err != nil {
		return err
	}

	ReleaseIP(ip, *subnet, fmt.Sprint(bridgeNetwork.VlanID))
	return nil
}

// createOvsInternalPort will generate a random name for the
// the port and ensure that it has been created
func createOvsInternalPort(prefix string, bridge string, tag uint) (port string, err error) {
	if port, err = GenerateRandomName(prefix, 7); err != nil {
		return
	}

	if ovsClient == nil {
		err = errors.New("OVS not connected")
		return
	}

	AddInternalPort(ovsClient, bridge, port, tag)
	return
}

// GenerateRandomName returns a new name joined with a prefix.  This size
// specified is used to truncate the randomly generated value
func GenerateRandomName(prefix string, size int) (string, error) {
	id := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, id); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(id)[:size], nil
}

func generateMacAddr(ip net.IP) net.HardwareAddr {
	hw := make(net.HardwareAddr, 6)

	// The first byte of the MAC address has to comply with these rules:
	// 1. Unicast: Set the least-significant bit to 0.
	// 2. Address is locally administered: Set the second-least-significant bit (U/L) to 1.
	// 3. As "small" as possible: The veth address has to be "smaller" than the bridge address.
	hw[0] = 0x02

	// The first 24 bits of the MAC represent the Organizationally Unique Identifier (OUI).
	// Since this address is locally administered, we can do whatever we want as long as
	// it doesn't conflict with other addresses.
	hw[1] = 0x42

	// Insert the IP address into the last 32 bits of the MAC address.
	// This is a simple way to guarantee the address will be consistent and unique.
	copy(hw[2:], ip.To4())

	return hw
}

func setupIPTables(bridgeName string, bridgeIP string) error {
	/*
		# Enable IP Masquerade on all ifaces that are not bridgeName
		# TO-DO need only one trunk interface as gw for per host only masquerade on that ip
		iptables -t nat -A POSTROUTING -s 10.1.42.1/16 ! -o %bridgeName -j MASQUERADE

		# disable outgoing connections on other vlan gateway
		iptables -A FORWARD -i %bridgeName ! -o %oldGateway -j DROP
	*/

	fmt.Println("Setting up iptables")
	natArgs := []string{"-t", "nat", "-A", "POSTROUTING", "-s", bridgeIP, "!", "-o", bridgeName, "-j", "MASQUERADE"}
	output, err := installRule(natArgs...)
	if err != nil {
		fmt.Println("Unable to enable network bridge NAT:", err)
		return fmt.Errorf("Unable to enable network bridge NAT: %s", err)
	}
	if len(output) != 0 {
		fmt.Println("Error enabling network bridge NAT:", output)
		return fmt.Errorf("Error enabling network bridge NAT: %s", output)
	}

	networks, _ := GetNetworks()
	for _, network := range networks {
		outboundArgs := []string{"-A", "FORWARD", "-i", bridgeName, "-o", network.Name, "-j", "DROP"}
		output, err = installRule(outboundArgs...)
		if err != nil {
			fmt.Println("Unable to disable network outbound forwarding:", err)
			return fmt.Errorf("Unable to disable network outbound forwarding: %s", err)
		}
		if len(output) != 0 {
			fmt.Println("Error disable network outbound forwarding:", output)
			return fmt.Errorf("Error disable network outbound forwarding: %s", output)
		}

	}

	return nil
}

func installRule(args ...string) ([]byte, error) {
	path, err := exec.LookPath("iptables")
	if err != nil {
		return nil, errors.New("iptables not found")
	}

	output, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("iptables failed: iptables %v: %s (%s)", strings.Join(args, " "), output, err)
	}

	return output, err
}

type notifier struct {
}

func (n notifier) Update(context interface{}, tableUpdates libovsdb.TableUpdates) {
}
func (n notifier) Locked([]interface{}) {
}
func (n notifier) Stolen([]interface{}) {
}
func (n notifier) Echo([]interface{}) {
}
func (n notifier) Disconnected(ovsClient *libovsdb.OvsdbClient) {
	fmt.Println("OVS Disconnected. Retrying...")
}

func addQos(containerId, bw, delay string) error {
	// use tc command to set container egress bw and delay
}

func installQos(args ...string) ([]byte, error) {
	path, err := exec.LookPath("tc")
	if err != nil {
		return nil, errors.New("tc not found")
	}

	output, err := exec.Command(path, args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tc failed: tc %v: %s (%s)", strings.Join(args, " "), output, err)
	}

	return output, err
}
