package netAgent

import (
	"bytes"
	"flag"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	flag.Set("alsologtostderr", "true")
	flag.Set("log_dir", "/tmp")
	flag.Set("v", "3")
	flag.Parse()

	ret := m.Run()
	os.Exit(ret)
}

func TestStartAgent(t *testing.T) {
	err := StartAgent(true, true, "", "data-dir")
	if err != nil {
		t.Error("Error starting Consul ", err)
	}
}

func TestJoin(t *testing.T) {
	err := Join("255.255.255.254")
	if err == nil {
		t.Error("Join to unknown peer must fail")
	}
}

func TestGet(t *testing.T) {
	existingVal, _, ok := Get("haha", "test")

	if ok {
		t.Fatal("error should not have value", string(existingVal[:]))
	}
}

func TestPut(t *testing.T) {
	existingVal, _, ok := Get("haha", "test")
	if ok {
		t.Fatalf("error, should not have value")
	}

	err := Put("haha", "test", []byte("192.168.1.1"), existingVal)
	if err != OK {
		t.Fatalf("Put failed")
	}

	err = Put("haha", "test", []byte("192.168.1.1"), existingVal)
	if err == OK {
		t.Fatalf("Put failed when there is no existing value")
	}

	existingVal, _, ok = Get("haha", "test")
	if !ok {
		t.Fatalf("test kv pair shoud exist")
	}

	err = Put("haha", "test", []byte("192.168.2.1"), existingVal)

	if err != OK {
		t.Errorf("Error putting value in store")
	}

	existingVal, _, ok = Get("haha", "test")

	if !ok {
		t.Fatalf("test key not found")
	}

	if !bytes.Equal(existingVal, []byte("192.168.2.1")) {
		t.Fatalf("Value is not right in the store")
	}
}

func TestCleanup(t *testing.T) {
	Delete("haha", "test")
	Leave()
}
