package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// test get version

func TestGetVersion(t *testing.T) {
	d := NewDaemon()

	request, _ := http.NewRequest("GET", "/version", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

// test conf related
func TestGetConfigurationEmpty(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("GET", "/configuration", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

func TestGetConfiguration(t *testing.T) {
	d := NewDaemon()
	// prepare the bridge conf
	d.bridgeConf = &BridgeConf{
		BridgeIP:   "172.16.42.1",
		BridgeName: "ovs-br0",
		BridgeCIDR: "172.16.42.0/24",
		BridgeMTU:  1440,
	}
	request, _ := http.NewRequest("GET", "/configuration", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	fmt.Println(response.Body)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

func TestSetConfigurationNoBody(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("POST", "/configuration", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("Expected %v:\n\tReceived: %v", "400", response.Code)
	}
}

func TestSetConfigurationBadBody(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("POST", "/configuration", bytes.NewReader([]byte{1, 2, 3, 4}))
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("Expected %v:\n\tReceived: %v", "500", response.Code)
	}
}

func TestSetConfiguration(t *testing.T) {
	d := NewDaemon()
	cfg := &BridgeConf{
		BridgeIP:   "172.16.42.1",
		BridgeName: "ovs-br0",
		BridgeCIDR: "172.16.42.0/24",
		BridgeMTU:  1440,
	}
	data, _ := json.Marshal(cfg)
	request, _ := http.NewRequest("POST", "/configuration", bytes.NewReader(data))
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

// test network related need start backend fot kv store
func TestGetNetworksApi(t *testing.T) {
	t.Skip("unable to mock network store")
	d := NewDaemon()
	request, _ := http.NewRequest("GET", "/networks", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

func TestGetNetworkApi(t *testing.T) {
	t.Skip("unable to mock network store")
	daemon := NewDaemon()
	/* ToDo: How do we inject this network?
	network := &Network{
		ID:      "foo",
		Subnet:  "10.10.10.0/24",
		Gateway: "10.10.10.1",
		Vlan:    uint(1),
	}
	*/

	request, _ := http.NewRequest("GET", "/network/foo", nil)
	response := httptest.NewRecorder()

	createRouter(daemon).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

func TestSetNetworksApi(t *testing.T) {
	t.Skip("unable to mock network store")
	daemon := NewDaemon()
	network := &Network{
		Name:    "foo",
		Subnet:  "10.10.10.0/24",
		Gateway: "10.10.10.1",
		VlanID:  uint(1),
	}
	data, _ := json.Marshal(network)

	request, _ := http.NewRequest("POST", "/network", bytes.NewReader(data))
	response := httptest.NewRecorder()

	createRouter(daemon).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

func TestDeleteNetworkApi(t *testing.T) {
	t.Skip("unable to mock network store")
	d := NewDaemon()
	/* ToDo: How do we inject this network?
	network := &Network{
		ID:      "foo",
		Subnet:  "10.10.10.0/24",
		Gateway: "10.10.10.1",
		Vlan:    uint(1),
	}
	*/

	request, _ := http.NewRequest("DELETE", "/networks", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

func TestGetNetworkNonExistentApi(t *testing.T) {
	t.Skip("unable to mock network store")
	d := NewDaemon()
	request, _ := http.NewRequest("GET", "/networks/abc123", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("Expected %v:\n\tReceived: %v", "404", response.Code)
	}
}

func TestDeleteNetworkNonExistentApi(t *testing.T) {
	t.Skip("unable to mock network store")
	d := NewDaemon()
	request, _ := http.NewRequest("DELETE", "/connections/abc123", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("Expected %v:\n\tReceived: %v", "404", response.Code)
	}
}

// test the node join and leave functionality
func TestClusterJoin(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("POST", "/cluster/join?address=1.1.1.1", nil)
	response := httptest.NewRecorder()

	go createRouter(d).ServeHTTP(response, request)
	foo := <-d.clusterChan
	if foo == nil {
		t.Fatal("object from clusterChan is nil")
	}

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

func TestClusterJoiniBadIp(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("POST", "/cluster/join?address=bar", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("Expected %v:\n\tReceived: %v", "500", response.Code)
	}
}

func TestClusterJoinNoParams(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("POST", "/cluster/join", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatal("request should fail")
	}
}

func TestClusterJoinBadParams(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("POST", "/cluster/join?foo!@£%£", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatal("request should fail")
	}
}

func TestClusterJoinBadParams2(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("POST", "/cluster/join?foo=bar", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatal("request should fail")
	}
}

func TestClusterLeave(t *testing.T) {
	d := NewDaemon()

	request, _ := http.NewRequest("POST", "/cluster/leave", nil)
	response := httptest.NewRecorder()

	go createRouter(d).ServeHTTP(response, request)

	foo := <-d.clusterChan
	if foo == nil {
		t.Fatal("object from clusterChan is nil")
	}

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

func TestGetConns(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("GET", "/connections", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}

func TestGetConn(t *testing.T) {
	d := NewDaemon()
	connection := &Connection{
		ContainerID:   "abc123",
		ContainerName: "test_container",
		ContainerPID:  "1234",
		Network:       "default",
		BandWidth:     "500",
		Delay:         "100",
	}
	d.connections["abc123"] = connection
	request, _ := http.NewRequest("GET", "/connection/abc123", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}

	expected, _ := json.Marshal(connection)
	if !bytes.Equal(response.Body.Bytes(), expected) {
		t.Fatal("body does not match")
	}

	headers := response.HeaderMap["Content-Type"]
	if headers[0] != "application/json; charset=utf-8" {
		t.Fatal("headers not correctly set")
	}
}

func TestCreateConn(t *testing.T) {
	d := NewDaemon()
	connection := &Connection{
		ContainerID:   "abc123",
		ContainerName: "test_container",
		ContainerPID:  "1234",
		Network:       "foo",
	}
	data, _ := json.Marshal(connection)
	request, _ := http.NewRequest("POST", "/connection", bytes.NewReader(data))
	response := httptest.NewRecorder()

	go func() {
		for {
			context := <-d.connectionChan
			if context == nil {
				t.Fatalf("Object taken from channel is nil")
			}
			if context.Action != addConn {
				t.Fatal("should be adding a new connection")
			}

			if !reflect.DeepEqual(context.Connection, connection) {
				t.Fatal("payload is incorrect")
			}
			context.Result <- connection
		}
	}()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v:\n\t%v", "200", response.Code, response.Body)
	}

	if !bytes.Equal(data, response.Body.Bytes()) {
		t.Fatalf("body is not correct")
	}

	contentHeader := response.HeaderMap["Content-Type"]
	if contentHeader[0] != "application/json; charset=utf-8" {
		t.Fatal("headers not correctly set")
	}
}

func TestCreateConnNoNetwork(t *testing.T) {
	d := NewDaemon()
	connection := &Connection{
		ContainerID:   "abc123",
		ContainerName: "test_container",
		ContainerPID:  "1234",
		Network:       "",
	}
	expected := &Connection{
		ContainerID:   "abc123",
		ContainerName: "test_container",
		ContainerPID:  "1234",
		Network:       "cxy",
	}

	data, _ := json.Marshal(connection)
	request, _ := http.NewRequest("POST", "/connection", bytes.NewReader(data))
	response := httptest.NewRecorder()

	go func() {
		for {
			context := <-d.connectionChan
			if context == nil {
				t.Fatalf("Object taken from channel is nil")
			}
			if context.Action != addConn {
				t.Fatal("should be adding a new connection")
			}

			if !reflect.DeepEqual(context.Connection, expected) {
				t.Fatal("payload is incorrect")
			}
			context.Result <- expected
		}
	}()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v:\n\t%v", "200", response.Code, response.Body)
	}

	expectedBody, _ := json.Marshal(expected)
	if !bytes.Equal(expectedBody, response.Body.Bytes()) {
		t.Fatalf("body is not correct")
	}

	contentHeader := response.HeaderMap["Content-Type"]
	if contentHeader[0] != "application/json; charset=utf-8" {
		t.Fatal("headers not correctly set")
	}
}

func TestCreateConnWithIP(t *testing.T) {
	d := NewDaemon()
	connection := &Connection{
		ContainerID:   "abc123",
		ContainerName: "test_container",
		ContainerPID:  "1234",
		Network:       "",
		RequestIp:     "10.10.10.10",
	}
	expected := &Connection{
		ContainerID:   "abc123",
		ContainerName: "test_container",
		ContainerPID:  "1234",
		Network:       "cxy",
		RequestIp:     "10.10.10.10",
	}

	data, _ := json.Marshal(connection)
	request, _ := http.NewRequest("POST", "/connection", bytes.NewReader(data))
	response := httptest.NewRecorder()

	go func() {
		for {
			context := <-d.connectionChan
			if context == nil {
				t.Fatalf("Object taken from channel is nil")
			}
			if context.Action != addConn {
				t.Fatal("should be adding a new connection")
			}

			if !reflect.DeepEqual(context.Connection, expected) {
				t.Fatal("payload is incorrect")
			}
			context.Result <- expected
		}
	}()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v:\n\t%v", "200", response.Code, response.Body)
	}

	expectedBody, _ := json.Marshal(expected)
	if !bytes.Equal(expectedBody, response.Body.Bytes()) {
		t.Fatalf("body is not correct")
	}

	contentHeader := response.HeaderMap["Content-Type"]
	if contentHeader[0] != "application/json; charset=utf-8" {
		t.Fatal("headers not correctly set")
	}
}

func TestCreateConnNoBody(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("POST", "/connection", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("Expected %v:\n\tReceived: %v", "400", response.Code)
	}
}

func TestCreateConnBadBody(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("POST", "/connection", bytes.NewReader([]byte{1, 2, 3, 4}))
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("Expected %v:\n\tReceived: %v", "500", response.Code)
	}
}

func TestGetConnNonExistent(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("GET", "/connection/abc123", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("Expected %v:\n\tReceived: %v", "404", response.Code)
	}
}

func TestDeleteConnNonExistent(t *testing.T) {
	d := NewDaemon()
	request, _ := http.NewRequest("DELETE", "/connection/abc123", nil)
	response := httptest.NewRecorder()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("Expected %v:\n\tReceived: %v", "404", response.Code)
	}
}

func TestDeleteConn(t *testing.T) {
	d := NewDaemon()
	connection := &Connection{
		ContainerID:   "abc123",
		ContainerName: "test_container",
		ContainerPID:  "1234",
		Network:       "default",
	}
	d.connections["abc123"] = connection
	request, _ := http.NewRequest("DELETE", "/connection/abc123", nil)
	response := httptest.NewRecorder()

	go func() {
		for {
			context := <-d.connectionChan
			if context == nil {
				t.Fatalf("Object taken from channel is nil")
			}
			if context.Action != deleteConn {
				t.Fatal("should be adding a new connection")
			}

			if !reflect.DeepEqual(context.Connection, connection) {
				t.Fatal("payload is incorrect")
			}
			context.Result <- connection
		}
	}()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}

}

// Qos test
func TestCreateQos(t *testing.T) {
	d := NewDaemon()
	connection := &Connection{
		ContainerID:   "abc123",
		ContainerName: "test_container",
		ContainerPID:  "1234",
		Network:       "default",
		BandWidth:     "500",
		Delay:         "100",
	}
	d.connections["abc123"] = connection
	request, _ := http.NewRequest("POST", "/qos/abc123?bw=500&delay=100", nil)
	response := httptest.NewRecorder()

	go func() {
		for {
			context := <-d.connectionChan
			if context == nil {
				t.Fatalf("Object taken from channel is nil")
			}
			if context.Action != deleteConn {
				t.Fatal("should be adding a new connection")
			}

			if !reflect.DeepEqual(context.Connection, connection) {
				t.Fatal("payload is incorrect")
			}
			context.Result <- connection
		}
	}()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}

}

/*
func TestUpdateQos(t *testing.T) {
	d := NewDaemon()
	connection := &Connection{
		ContainerID:   "abc123",
		ContainerName: "test_container",
		ContainerPID:  "1234",
		Network:       "default",
	}
	d.connections["abc123"] = connection
	request, _ := http.NewRequest("DELETE", "/connection/abc123", nil)
	response := httptest.NewRecorder()

	go func() {
		for {
			context := <-d.connectionChan
			if context == nil {
				t.Fatalf("Object taken from channel is nil")
			}
			if context.Action != deleteConn {
				t.Fatal("should be adding a new connection")
			}

			if !reflect.DeepEqual(context.Connection, connection) {
				t.Fatal("payload is incorrect")
			}
			context.Result <- connection
		}
	}()

	createRouter(d).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected %v:\n\tReceived: %v", "200", response.Code)
	}
}*/
