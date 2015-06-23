#!/bin/bash

# simple test.sh when doing code change test
set -e
make build
sudo ovs-vsctl --if-exists del-br ovs-br0
echo y | sudo ./client agent restart
sleep 5
sudo ./client network create test 10.10.1.0/24
sudo ./client network list