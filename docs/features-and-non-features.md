### Features

* connects multiple nodes using wireguard
* fully automated exchange of wireguard endpoint, public key
* IP address management for mesh nodes
* TLS for gRPC endpoints
* keeps track of nodes in case of node failures, automatically removing wireguard peers
* keeps track of new nodes joining the mash, automatically adding peer data

### Non-Features

* no subnet support. All nodes set individual /32 host routes to all other nodes. 
* it does not do any routing by a daemon
* it does not guarantee any NAT support. E.g. if two nodes are behind a NAT, they typically cannot connect to each other
