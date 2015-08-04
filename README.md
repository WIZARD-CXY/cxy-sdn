# CXY-SDN

[![Travis](https://travis-ci.org/WIZARD-CXY/cxy-sdn.svg?branch=master)](https://travis-ci.org/WIZARD-CXY/cxy-sdn)

Using sdn tools and netAgent to offer advanced network management for container cloud platform.

Version 7.0 ready!

## Feature

1 New node automatically joins the cluster and configures the network settings.

2 Support multiple networks (VLAN).

3 Use consul as data store backend and make all the configuration data available on every node.

4 Build a cxy-sdn docker image, so easy to run.

5 Can control the container network QoS settings like bandwidth and latency.

6 Monitor the container network traffic, clients can get historical or instantaneous ingress and egress rate via http request.

7 Migrate containers among different hosts without changing ip address.

8 Support k8s as a network plugin !!