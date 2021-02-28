# wgmesh
Automatically build private wireguard mesh networks.

[![Go Report Card](https://goreportcard.com/badge/github.com/aschmidt75/wgmesh)](https://goreportcard.com/report/github.com/aschmidt75/wgmesh)
[![Go](https://github.com/aschmidt75/wgmesh/actions/workflows/go.yml/badge.svg)](https://github.com/aschmidt75/wgmesh/actions/workflows/go.yml)

## How it works

* A mesh consist of interconnected nodes. Each node has a [wireguard](https://www.wireguard.com/) interface with all other nodes registered as peers. The mesh is thus fully-connected through the wireguard-based overlay network. Each node can IP-reach all other nodes via direct host routes.
* At least one of the mesh nodes is a bootstrap node. Besides the wireguard peerings it runs a gRPC-based mesh endpoint, where other nodes can issue join requests.
* New nodes can enter the mesh by joining via a bootstrap node. The bootstrap node learns about the joining node and its wireguard endpoint and public key. It distributes this information to the other mesh nodes. It also fowards a list of mesh peers to the new joining node so it is able to configure its own wireguard interface.
* Existing mesh nodes learn about the new joining node from the bootstrap node and update their wireguard peer configuration accordingly.
* Mesh nodes integrate [serf.io](https://serf.io) to maintain state about the network topology, learn about new nodes joining and failing nodes or nodes leaving the mesh. This is done using serf's encrypted gossip-based cluster membership protocol.
* The bootstrap node's gRPC endpoint runs TLS with requesting and validating client certificates. This way new nodes are required to authenticate themselves by x.509 certificates. 

## Build

The default targets of `Makefile` generate protobug and grpc parts and build the binary in `dist/`

```bash
$ make
```

Additionally, goreleaser can be used to create a snapshot release for different platforms (also in `dist/`)

```bash
$ make release
```

## Prerequisites

* Linux
* Wireguard module installed/enabled in kernel
* Works best with a non-NATed setup. Usually works with NAT, but limitations may apply.

## Use

The fastest way to start a mesh is using the development mode, either with a bunch of local virtual machine or with available cloud instances.

The mesh is initiated with a first bootstrap node. It creates a wireguard interface and starts listening for join requests on a gRPC endpoint. Make sure that the bootstrap node is not behind a NAT. In development mode, no security/TLS/mesh encryption is enforced, so all other nodes can join with authentication. This simplifies testing but is not suitable for non-development purposes.

```bash
# wgmesh bootstrap -dev 

** Mesh name:                       xoJbYw07PM
** Mesh CIDR range:                 10.232.0.0/16
** gRPC Service listener endpoint:  0.0.0.0:5000
** This node's name:                xoJbYw07PMAE80101
** This node's mesh IP:             10.232.1.1
**
** This mesh is running in DEVELOPMENT MODE without encryption.
** Do not use this in a production setup.
**
** To have another node join this mesh, use this command:
** wgmesh join -v -dev -n xoJbYw07PM -bootstrap-addr <PUBLIC_IP_OF_THIS_NODE>:5000
**
** To inspect the wireguard interface and its peer data use:
** wg show wgxoJbYw07PM
**
** To inspect the current mesh status use: wgmesh info
**
```

For other nodes to join, the (public or private) IP address of the bootstrap node is needed, so joining nodes are able to connect. Switch to a second instance and run the join command as stated above:

```bash
# wgmesh join -v -dev -n xoJbYw07PM -bootstrap-addr 10.0.0.0:5000

INFO[2021/02/26 15:46:31] Fetching external IP from STUN server
INFO[2021/02/26 15:46:31] Using external IP when connecting with mesh   ip=
INFO[2021/02/26 15:46:31] Created and configured wireguard interface wgxoJbYw07PM as no-up
WARN[2021/02/26 15:46:31] Using insecure connection to gRPC mesh service
INFO[2021/02/26 15:46:32] Starting gRPC Agent Service at /var/run/wgmesh.sock
**
** Mesh 'xoJbYw07PM' has been joined.
**
** Mesh name:                       xoJbYw07PM
** Mesh CIDR range:                 10.232.0.0/16
** This node's name:                xoJbYw07PMAE8B8DB
** This node's mesh IP:             10.232.184.219
**
** This mesh is running in DEVELOPMENT MODE without encryption.
** Do not use this in a production setup.
**
** To inspect the wireguard interface and its peer data use:
** wg show wgxoJbYw07PM
**
** To inspect the current mesh status use: wgmesh info
**
INFO[2021/02/26 15:46:33] Mesh has 2 nodes
```

Additional nodes can join using the same `join` command.

On any node, the `info` command prints out connected nodes:

```bash
# wgmesh info
Mesh 'xoJbYw07PM' has 2 nodes, started 2021-02-27 10:45:31 +0100 CET
This node 'xoJbYw07PMAE8B8DB' joined 2021-02-27 10:46:31 +0100 CET

Name              |Address        |Status |RTT |Tags                                |
xoJbYw07PMAE8B8DB |10.232.184.219 |alive  |7   | _addr=, _port=54540,  |
xoJbYw07PMAE80101 |10.232.1.1     |alive  |38  | _addr=, _port=54540, |
```



## License

(C) 2020,2021 @aschmidt75 
Apache License, Version 2.0
