Here I briefly discuss how to deploy cxy-sdn along with Kubernetes, tested on Kubernetes 1.0.6

First, cxy-sdn has two parts: client side(cxy_sdn, bash script) and a server side(advanced network management and configuration tool wrapped in a docker image which name is wizardcxy/cxy-sdn)

Deploy procedure on every Kubernetes minion node:

1 ` $ sudo mkdir -p /usr/libexec/kubernetes/kubelet-plugins/net/exec/cxy~cxy_sdn/ && sudo cp cxy_sdn /usr/libexec/kubernetes/kubelet-plugins/net/exec/cxy~cxy_sdn/`.

2 `$ sudo ./cxy_sdn agent start (you may first meet all the dependencies by run $ sudo ./cxy_sdn deps).
  If it is the first node, hit 'y' and continue, else hit 'n' and use '$ sudo ./cxy_sdn cluster join $THE_FIRST_NODE_IP`

3 Add cmd option '--network-plugin=cxy/cxy_sdn' for kubelet and restart kubelet.

4 Check kubelet log to see if the plugin is correctly loaded.

5 `$ sudo ./cxy_sdn agent logs` to see if cxy_sdn works well.