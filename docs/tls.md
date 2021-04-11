# TLS mode

## CA setup and certification generation

e.g. using cfssl. Must have a CA certificate (i.e. of an intermediate), a key and cert file for bootstrap node
with correct hostname (and SANs), and key and cert file for joining node(s). All key material must be provided
in PEM format. 

## bootstrap

```bash
# wgmesh bootstrap -v \
    -grpc-ca-cert /path/to/ca.pem \
    -grpc-server-cert /path/to/bootstrap.pem \
    -grpc-server-key /path/to/bootstrap-key.pem
```

```bash
 # wgmesh join -v \
    -ca-cert /root/wgmesh-tls/ca.pem \
    -client-cert /root/wgmesh-tls/join.pem \
    -client-key /root/wgmesh-tls/join-key.pem \
    -n okOJGomAHM \
    -bootstrap-addr <bootstrap-node-ip>:5000
```

