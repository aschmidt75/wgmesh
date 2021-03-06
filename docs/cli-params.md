
## CLI Usage and Parameters

`wgmesh` understands the following commands:

* `bootstrap` is used to start a node in bootstrap mode. At least one node in a mesh has to be a bootstrap node, so that other nodes are able to join. This command will run in foreground. It can be stopped with CTRL-C or otherwise terminating/killing it. Mesh connectivity is maintained as long as the command is running.
* `join` is used to join an existing mesh by connecting to a bootstrap node. This command will run in foreground and maintain mesh connectivity as long as it is running. 
* `info` prints out information about the mesh and its nodes. It can be used on bootstrapped or joined nodes where one of the above commands is running.
* `tags` is used to set or remove tags on the current node.
* `rtt` prints out a table of round-trip-times for all nodes.

### Common parameter for all commands

* `-v` enables verbose mode which shows more ouput, e.g. nodes joining or leaving
* `-d` enables debug mode, showing much more output of internal state changes etc.

### Common parameters for `bootstrap`/`join`

* `dev` enables **DEVELOPMENT** mode. In this mode, no encryption is in place, no TLS setup needs to be specified. **This is suitable for trying things out but leaves your mesh more or less open for everyone to access it. Do not use this for non-development setups.**
* `name`, `n` (optional) name of mesh to bootstrap or join. This can have a maximum length of 10 characters, as it will be used to form the interface and node names for wireguard. If left empty, wgmesh will assign a random 10-char string.
* `node-name` (optional) name of the current node. It is advised to keep it short as it will be used in gossip traffic between nodes. If left empty, wgmesh will automatically assign a name combined of the mesh name and the internal node ip.
* `listen-addr` Endpoint IP address where the wireguard interface is listening for UDP packets. May be specified as an IP address or as a interface name. If left empty, wgmesh uses STUN to determine the hosts' public ip. Leaving this parameter empty makes sense in non-NATed and NATed environment with public IP addresses. It does not make sense in private network setups, IP addresses should be explicitly given here.
* `listen-port` (default 54540) UDP port of the wireguard endpoint
* `agent-bind-socket` is a path to the socket file where the local wgmesh agent serves gRPC requests, such as the `info` or `tags` commands
* `agent-bind-socket-id` is of the form UID:GID and is used to chown the above agent-bind-socket file to this user id and group id. 
* `memberlist-file` points to a JSON file where wgmesh stores up-to-date information about the current mesh topology. Every time nodes enter or leave the mesh, or tags are updated, this file gets rewritten.

### `bootstrap`

This command is for bootstrap nodes only.

* `ip` (default 10.232.1.1 for bootstrap nodes, not used for joining nodes) private IP of this node. This needs to be specified for the bootstrap nodes. wgmesh will take this ip address on the wireguard interface for itself.
* `cidr` (default 10.232.0.0/16) This is the private network in CIDR notation. All nodes joining the mesh will be assigned an IP address within this address space.
* `grpc-bind-addr` (default 0.0.0.0 only applies for bootstrap nodes) Bind address for the public gRPC service of bootstrap nodes
* `grpc-bind-port`(default 5000 only for bootstrap nodes) TCP port number for the public gRPC service
* `grpc-server-key` points to the PEM-encoded private key. This is used for the external gRPC service.
* `grpc-server-cert` points to the PEM-encoded certificate.
* `grpc-ca-cert` points to a PEM-encoded CA certificate used to authenticate joining nodes. Mutually exlusive with `grpc-ca-path`
* `grpc-ca-path` points to a directory where PEM-encoded certificates reside. They are used to authenticate joining nodes. Mutually exlusive with `grpc-ca-cert`
* `mesh-encryption-key` (optional) base64-encoded, 32 bytes symmetric encryption key used to encrypt internal mesh traffic. If this is left out, wgmesh will assign a randomized key. 
* `serf-mode-lan` if set to true, use the LAN mode defaults for Serf, otherwise use the WAN mode defaults (e.g. timeouts, fan-outs etc.). This is set on the bootstrap node only and will be propagated to joining nodes.

### `join`

This command is for joining nodes only

* `bootstrap-addr` (mandatory, only for joining nodes) Address (IP:port) of the gRPC service of a bootstrap node. 
* `client-key` points to the PEM-encoded private key. This is used when connecting to the bootstrap node. 
* `client-cert` points to the PEM-encoded certificate. This must be recognized by the bootstrap mode (see there `grpc-ca-cert` or  `grpc-ca-path`)
* `ca-cert` points to a PEM-encoded CA certificate 

### `info`

* `agent-grpc-socket` is the socket file, see above `agent-bind-socket`.
* `watch` keeps running in foreground and prints out mesh information everytime a change occures within the mesh topology.

### `tags`

* `agent-grpc-socket` is the socket file, see above `agent-bind-socket`.
* `set` is used to set a tag as a key=value pair on the current node. This tag will be propagated to all nodes in the mesh.
* `delete` removes a tag from the current node. Tag removal will be propagated to all mesh nodes.

### `rtt`

* `agent-grpc-socket` is the socket file, see above `agent-bind-socket`.

