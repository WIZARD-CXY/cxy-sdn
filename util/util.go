package util

import (
	"errors"
	"fmt"

	"net"

	// log "github.com/golang/glog"
	"github.com/vishvananda/netlink"
	"math"
)

var (
	ErrNoDefaultRoute                 = errors.New("no default route")
	ErrNetworkOverlapsWithNameservers = errors.New("requested network overlaps with nameserver")
	ErrNetworkOverlaps                = errors.New("requested network overlaps with existing network")
)

func CheckRouteOverlaps(toCheck *net.IPNet) error {
	networks, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	for _, network := range networks {
		if network.Dst != nil && NetworkOverlaps(toCheck, network.Dst) {
			return ErrNetworkOverlaps
		}
	}
	return nil
}

// Detects overlap between one IPNet and another
func NetworkOverlaps(netX *net.IPNet, netY *net.IPNet) bool {
	if firstIP, _ := NetworkRange(netX); netY.Contains(firstIP) {
		return true
	}
	if firstIP, _ := NetworkRange(netY); netX.Contains(firstIP) {
		return true
	}
	return false
}

// Calculates the first and last IP addresses in an IPNet
func NetworkRange(network *net.IPNet) (net.IP, net.IP) {
	var (
		netIP   = network.IP.To4()
		firstIP = netIP.Mask(network.Mask)
		lastIP  = net.IPv4(0, 0, 0, 0).To4()
	)

	for i := 0; i < len(lastIP); i++ {
		lastIP[i] = netIP[i] | ^network.Mask[i]
	}
	return firstIP, lastIP
}

// Return the IPv4 address of a network interface
func GetIfaceAddr(name string) (*net.IPNet, error) {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return nil, err
	}

	addrs, err := netlink.AddrList(iface, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}

	if len(addrs) == 0 {
		return nil, fmt.Errorf("Interface %v has no IP addresses", name)
	}

	if len(addrs) > 1 {
		// log.Info("Interface %v has more than 1 IPv4 address. Defaulting to using %v\n", name, addrs[0].IP)
	}

	return addrs[0].IPNet, nil
}

func GetDefaultRouteIface() (int, error) {
	defaultRt := net.ParseIP("0.0.0.0")
	rs, err := netlink.RouteGet(defaultRt)
	if err != nil {
		return -1, fmt.Errorf("unable to get default route: %v", err)
	}
	if len(rs) > 0 {
		return rs[0].LinkIndex, nil
	}
	return -1, ErrNoDefaultRoute
}

func InterfaceUp(name string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkSetUp(iface)
}

func InterfaceDown(name string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkSetDown(iface)
}

func ChangeInterfaceName(old, newName string) error {
	iface, err := netlink.LinkByName(old)
	if err != nil {
		return err
	}
	return netlink.LinkSetName(iface, newName)
}

func SetInterfaceInNamespacePid(name string, nsPid int) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkSetNsPid(iface, nsPid)
}

func SetInterfaceInNamespaceFd(name string, fd uintptr) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkSetNsFd(iface, int(fd))
}

func SetDefaultGateway(ip, ifaceName string) error {
	fmt.Println("haha set defaultGateway", ip, ifaceName)
	iface, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return err
	}
	fmt.Println("haha2 set defaultGateway", ip, ifaceName)
	gw := net.ParseIP(ip)
	if gw == nil {
		return errors.New("Invalid gateway address")
	}

	_, dst, err := net.ParseCIDR("0.0.0.0/0")
	if err != nil {
		return err
	}
	defaultRoute := &netlink.Route{
		LinkIndex: iface.Attrs().Index,
		Dst:       dst,
		Gw:        gw,
	}
	return netlink.RouteAdd(defaultRoute)
}

func SetInterfaceMac(name string, macaddr string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	hwaddr, err := net.ParseMAC(macaddr)
	if err != nil {
		return err
	}
	return netlink.LinkSetHardwareAddr(iface, hwaddr)
}

func SetInterfaceIp(name string, rawIp string) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}

	ipNet, err := netlink.ParseIPNet(rawIp)
	if err != nil {
		return err
	}
	addr := &netlink.Addr{ipNet, ""}
	return netlink.AddrAdd(iface, addr)
}

func SetMtu(name string, mtu int) error {
	iface, err := netlink.LinkByName(name)
	if err != nil {
		return err
	}
	return netlink.LinkSetMTU(iface, mtu)
}

func GetIfaceForRoute(address string) (string, error) {
	addr := net.ParseIP(address)
	if addr == nil {
		return "", errors.New("invalid address")
	}
	routes, err := netlink.RouteGet(addr)
	if err != nil {
		return "", err
	}
	if len(routes) <= 0 {
		return "", errors.New("no route to destination")
	}
	link, err := netlink.LinkByIndex(routes[0].LinkIndex)
	if err != nil {
		return "", err
	}
	return link.Attrs().Name, nil
}

// set the given bit, 0 index based
func set(a []byte, k uint32) {
	a[k/8] |= 1 << (k % 8)
}

// clear the given bit, 0 index based
func Clear(a []byte, k uint) {
	a[k/8] &= ^(1 << (k % 8))
}

// test whether the given bit is 1, 0 index based
func test(a []byte, k uint32) bool {
	return ((a[k/8] & (1 << (k % 8))) != 0)
}

// get the smallest 0 bit index and set it
// return its index, 1 based
// return len(a)*8+1 as all bits are set
func TestAndSet(a []byte) uint32 {
	var i uint32

	for i = 0; i < uint32(len(a)*8); i++ {
		if !test(a, i) {
			set(a, i)
			return i + 1
		}
	}
	return i + 1
}

// count how many IP available in the given subnet
func IPCount(subnet net.IPNet) float64 {
	maskSize, _ := subnet.Mask.Size()

	if subnet.IP.To4() != nil {
		return math.Pow(2, float64(32-maskSize))
	} else {
		return math.Pow(2, float64(128-maskSize))
	}

}
