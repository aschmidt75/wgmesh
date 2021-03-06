## Tags

Thanks to serf, all nodes in the mesh carry key/value parameters named "tags". wgmesh uses these tags to store additional information about nodes for both internal workings and added functionality as well.

### Internal wgmesh tags

Tags that start with an underscore are internal tags. They are not shown by `wgmesh info`, but can be accessed. The following tags are used internally:

* `_port` is the wireguard listen port
* `_addr` is the wireguard listen address
* `_pk` is the wireguard public key
* `_i` is the mesh-internal IP address of the node
* `_t` stores the node type: `b` for bootstrap nodes, `n` otherwise

### Setting tags using the CLI

The `tags` command can be used on any node to set or delete a tag for that node. It connects to the local gRPC service and modifies the tag list as desired, which is then automatically broadcasted by serf to all mesh nodes.

```bash
# wgmesh tags --help
Usage of tags:
  -agent-grpc-socket string
    	agent socket to dial (default "/var/run/wgmesh.sock")
  -d	show debug output
  -delete string
    	to delete a key
  -set string
    	set tag key=value
  -v	show more output
```

e.g. to set a tag use the `-set` option with a key/value pair

```bash
# wgset tags -set=node_size=small
```

will set the tag `node_size` to the value `small` on the node where it is executed.

To delete a tag, use the `-delete` option with a key:
```bash
# wgset tags -delete=node_size
```

To show all tags for this node, use the `tags` command without options.

### Reserved tag names

Besides the internal tag names, the following tag names are reserved:

#### Services

If a tag key begins with `svc:` it is treated as a mesh service, e.g. when exporting mesh node information.
The tag string is interpreted as a comma-separated list of key=value pairs, with the following keys:

* `port` is the port number where a service may be announced on a node.

*Example*

* key `svc:nginx` and value `port=80`
