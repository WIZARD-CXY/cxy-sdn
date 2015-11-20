#!/bin/bash
# simple test.sh when doing functional test

set -e
make build
sudo ovs-vsctl --if-exists del-br ovs-br0
echo y | sudo ./cxy_sdn agent restart
#wait a little while
sleep 10
sudo ./cxy_sdn network create test 10.10.1.0/24
