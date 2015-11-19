Here I briefly discuss how to deploy cxy-sdn along with Kubernetes, tested on Kubernetes 1.0.6

First, cxy-sdn have two parts: client side(cxy_sdn, bash script) and a server side(a network management and configuration tool wrapped in a docker image which name is wizardcxy/cxy-sdn)

Deploy procedure on every kubernetes minion node:
1 '$ sudo mkdir -p /usr/libexec/kubernetes/kubelet-plugins/net/exec/cxy~cxy_sdn/ && mv cxy_sdn /usr/libexec/kubernetes/kubelet-plugins/net/exec/cxy~cxy_sdn/'.
2 $ sudo ./cxy_sdn agent start (you may first meet all the dependencies by run $ sudo ./cxy_sdn deps).
3 Add cmd option '--network-plugin=cxy/cxy_sdn' for kubelet and restart kubelet.
4 Check kubelet log to see if the plugin is correctly loaded.
5 '$ sudo ./cxy_sdn agent logs' to see if cxy_sdn works well.