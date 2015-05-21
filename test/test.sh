#!/bin/bash

set -x
sudo service kubelet stop
kubectl delete pods sshd
sudo service docker restart
echo y | sudo scripts/socketplane.sh install
sleep 5
sudo socketplane agent logs
sudo socketplane network create test 10.2.0.0/16
sudo socketplane network list
sudo service kubelet start
kubectl create -f ~/misc/conf/sshd.yaml

sleep 10
docker ps
