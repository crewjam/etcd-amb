
# etcd ambassador

CoreOS describe a production architecture with a small number (<= 9) of etcd
nodes while the remaining nodes are etcd clients. Unfortunately neither CoreOS
nor etcd provide a way for slaves to discover the set of etcd servers.

https://coreos.com/docs/cluster-management/setup/cluster-architectures/#production-cluster-with-central-services

The problem is that there are zillions of places where it is assumed that etcd
is available at 127.0.0.1:4001. Rather than fight with configuration, we have
an ambassador that proxies access to the etcd ports on localhost to the real
etcd servers.

This program polls the global etcd discovery service to build a list of etcd
peers and set up the proxy (haproxy) appropriately.

An example unit file:

    [Unit]
    Description=etcd discovery container
    Before=fleet.service
    Conflicts=etcd.service

    [Service]
    ExecStartPre=-/usr/bin/docker kill etcd-amb
    ExecStartPre=-/usr/bin/docker rm etcd-amb
    ExecStartPre=/usr/bin/docker pull crewjam/etcd-amb
    ExecStart=/usr/bin/docker run --rm --name etcd-amb \
      -p 4001 -p 2379 -p 2380 \
      crewjam/etcd-amb -discovery-url=https://discovery.etcd.io/xxxxx \
    ExecStop=/usr/bin/docker kill etcd-amb
