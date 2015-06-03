package server

import (
	"flag"
	"fmt"
	"math/rand"
	"net"
	"os"
	"testing"
)

var subnetArray []*net.IPNet

// set glog flag to make it happy

func TestMain(m *testing.M) {
	flag.Set("alsologtostderr", "true")
	flag.Set("log_dir", "/tmp")
	flag.Set("v", "3")
	flag.Parse()

	ret := m.Run()
	os.Exit(ret)
}

func TestNetworkInit(t *testing.T) {
	if os.Getuid() != 0 {
		msg := "Skipped test because it requires root privileges."
		fmt.Println(msg)
		t.Skip(msg)
	}
	_, ipNet1, _ := net.ParseCIDR("192.168.1.0/24")
	_, ipNet2, _ := net.ParseCIDR("192.168.2.0/24")
	_, ipNet3, _ := net.ParseCIDR("192.168.3.0/24")
	_, ipNet4, _ := net.ParseCIDR("192.168.4.0/24")
	_, ipNet5, _ := net.ParseCIDR("192.168.5.0/24")

	subnetArray = []*net.IPNet{ipNet1, ipNet2, ipNet3, ipNet4, ipNet5}
}

func TestGetEmptyNetworks(t *testing.T) {
	networks, _ := GetNetworks()
	if networks == nil {
		t.Error("GetNetworks must return an empty array when networks are not created ")
	}
}

func TestNetworkCreate(t *testing.T) {
	if os.Getuid() != 0 {
		msg := "Skipped test because it requires root privileges."
		fmt.Println(msg)
		t.Skip(msg)
	}
	for i := 0; i < len(subnetArray); i++ {
		_, err := CreateNetwork(fmt.Sprintf("Network-%d", i+1), subnetArray[i])
		if err != nil {
			t.Error("Error Creating network ", err)
		}
		fmt.Println("Network ", i+1, "Created Successfully")
	}
}

func TestGetNetworks(t *testing.T) {
	networks, _ := GetNetworks()
	if networks == nil || len(networks) < len(subnetArray) {
		t.Error("GetNetworks must return an empty array when networks are not created ")
	}
}

func TestGetNetwork(t *testing.T) {
	if os.Getuid() != 0 {
		msg := "Skipped test because it requires root privileges."
		fmt.Println(msg)
		t.Skip(msg)
	}
	for i := 0; i < len(subnetArray); i++ {
		network, _ := GetNetwork(fmt.Sprintf("Network-%d", i+1))
		if network == nil {
			t.Error("Error GetNetwork")
		} else if network.Subnet != subnetArray[i].String() {
			t.Error("Network mismatch")
		}
		fmt.Println("GetNetwork : ", network)
	}
}

func TestNetworkCleanup(t *testing.T) {
	if os.Getuid() != 0 {
		msg := "Skipped test because it requires root privileges."
		fmt.Println(msg)
		t.Skip(msg)
	}
	for i := 0; i < len(subnetArray); i++ {
		err := DeleteNetwork(fmt.Sprintf("Network-%d", i+1))
		if err != nil {
			t.Error("Error Deleting Network", err)
		}
	}
}

func TestInitAgent(t *testing.T) {
	err := InitAgent("eth1", true)

	if err != nil {
		t.Errorf("Error starting agent")
	}
}

func TestRequestandReleaseIP(t *testing.T) {
	TestCount := rand.Intn(500) + 50

	_, ipNet, _ := net.ParseCIDR("192.168.0.0/16")

	for i := 1; i <= TestCount; i++ {
		addr := RequestIP(*ipNet)
		addr = addr.To4()
		if i%256 != int(addr[3]) || i/256 != int(addr[2]) {
			t.Error("addr.String()", "is wrong")
		}
	}

	if !ReleaseIP(net.ParseIP("192.168.0.1"), *ipNet) {
		t.Error("Release 192.168.0.1 failed")
	}
	if !ReleaseIP(net.ParseIP("192.168.0.4"), *ipNet) {
		t.Error("Release 192.168.0.4 failed")
	}

	addr := RequestIP(*ipNet).To4()
	if int(addr[3]) != 1 {
		t.Error(addr.String())
	}

	addr = RequestIP(*ipNet).To4()

	if int(addr[3]) != 4 {
		t.Error(addr.String())
	}
}
