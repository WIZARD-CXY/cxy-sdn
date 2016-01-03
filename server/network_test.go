package server

import (
	"fmt"
	"net"
	"os"
	"testing"
	_ "time"
)

var subnetArray []*net.IPNet
var bridgeUUID string

func TestStartAgent(t *testing.T) {
	err := InitAgent("eth0", true)

	if err != nil {
		t.Errorf("Error starting agent")
	}
}

func TestNetworkInit(t *testing.T) {
	if os.Getuid() != 0 {
		msg := "Skipping TestNetworkInit because it requires root privileges."
		fmt.Println(msg)
		t.Skip(msg)
	}

	//prepare the net Array
	_, ipNet1, _ := net.ParseCIDR("10.10.1.0/24")
	_, ipNet2, _ := net.ParseCIDR("10.10.2.0/24")
	_, ipNet3, _ := net.ParseCIDR("10.10.3.0/24")
	_, ipNet4, _ := net.ParseCIDR("10.10.4.0/24")

	subnetArray = []*net.IPNet{ipNet1, ipNet2, ipNet3, ipNet4}

	//create the ovs bridge
	if bridgeUUID, err := CreateBridge(); err != nil {
		t.Fatalf("Creat ovs bridge failed")
	} else {
		fmt.Printf("Bridge %s created\n", bridgeUUID)
	}
}

func TestGetEmptyNetworks(t *testing.T) {
	if os.Getuid() != 0 {
		msg := "Skipping TestNetworkInit because it requires root privileges."
		fmt.Println(msg)
		t.Skip(msg)
	}
	networks, _ := GetNetworks()
	if networks == nil {
		t.Error("GetNetworks must return an empty array when networks are not created ")
	}
}

func TestNetworkCreate(t *testing.T) {
	if os.Getuid() != 0 {
		msg := "Skipping TestNetworkCreate because it requires root privileges."
		fmt.Println(msg)
		t.Skip(msg)
	}
	for i := 0; i < len(subnetArray); i++ {
		_, err := CreateNetwork(fmt.Sprintf("Network-%d", i+1), subnetArray[i])
		if err != nil {
			t.Error("Error Creating network ", err)
		}
		fmt.Println("Network", i+1, "Created Successfully")
	}
}

func TestGetNetwork(t *testing.T) {
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
		msg := "Skipped TestNetworkCleanup test because it requires root privileges."
		fmt.Println(msg)
		t.Skip(msg)
	}
	for i := 0; i < len(subnetArray); i++ {
		err := DeleteNetwork(fmt.Sprintf("Network-%d", i+1))
		if err != nil {
			t.Error("Error Deleting Network", err)
		}
	}

	// delete the ovs bridge
	if err := DeleteBridge(); err != nil {
		t.Error("Delete ovs bridge failed", err)
	}
}

func TestAllocateandReleaseVNI(t *testing.T) {
	for i := 1; i <= 10; i++ {
		vni, err := allocateVNI()

		if err != nil || vni != uint(i) {
			t.Error("1.allocate vni wrong at", vni, i)
		}
	}
	for i := 1; i <= 5; i++ {
		releaseVNI(uint(2 * i))
	}

	for i := 1; i <= 5; i++ {
		vni, err := allocateVNI()
		if err != nil || vni != uint(2*i) {
			t.Error("2.allocate vni wrong at", vni, i)
		}
	}
}

func TestRequestandReleaseIP(t *testing.T) {
	TestCount := 5

	_, ipNet, _ := net.ParseCIDR("192.168.0.0/16")

	for i := 1; i <= TestCount; i++ {
		addr := RequestIP("1", *ipNet)
		addr = addr.To4()
		if addr == nil || i%256 != int(addr[3]) || i/256 != int(addr[2]) {
			t.Error(addr.String(), "is wrong")
		}
	}

	if !ReleaseIP(net.ParseIP("192.168.0.1"), *ipNet, "1") {
		t.Error("Release 192.168.0.1 failed")
	}
	if !ReleaseIP(net.ParseIP("192.168.0.4"), *ipNet, "1") {
		t.Error("Release 192.168.0.4 failed")
	}
	if !ReleaseIP(net.ParseIP("192.168.0.2"), *ipNet, "1") {
		t.Error("Release 192.168.0.2 failed")
	}

	addr := RequestIP("1", *ipNet).To4()
	if int(addr[3]) != 1 {
		t.Error(addr.String())
	}

	MarkUsed("1", net.ParseIP("192.168.0.2"), *ipNet)

	addr = RequestIP("1", *ipNet).To4()

	if int(addr[3]) != 4 {
		t.Error(addr.String())
	}
}

func TestLeaveCluster(t *testing.T) {
	if err := leave(); err != nil {
		t.Error("Error leaving the cluster")
	}
}
