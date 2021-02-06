# wgmesh
Automatically build private mesh networks using wireguard and serf.io


## CLI Parameters

### General

* `name`, `n` (mandatory) name of mesh to bootstrap of join. Max length 10 characters
* `cidr` (default 10.232.0.0/16) This is the private network in CIDR notation
* `listen-addr` Endpoint IP address where the wireguard interface is listening for UDP packets. May be specified as an IP address or as a interface name.
* `listen-port` (default 54540) UDP port of the wireguard endpoint

### Bootstrap nodes only

* `ip` (default 10.232.1.1 for bootstrap nodes, not used for joiner nodes) private IP of a node
* `grpc-bind-addr` (default 0.0.0.0 only applies for bootstrap nodes) Bind address for the public gRPC service of bootstrap nodes
* `grpc-bind-port`(default 5000 only for bootstrap nodes) TCP port number for the public gRPC service

### Joiner nodes only

* `bootstrap-addr` (mandatory, only for joiner nodes) Address (IP:port) of the gRPC service of a bootstrap node. 