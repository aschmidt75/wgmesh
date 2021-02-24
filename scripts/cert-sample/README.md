#

```bash
$ cfssl gencert -initca ca-csr.json | cfssljson -bare ca
$ cfssl gencert -ca ca.pem -ca-key ca-key.pem bootstrap-csr.json | cfssljson -bare bootstrap
$ cfssl gencert -ca ca.pem -ca-key ca-key.pem join-csr.json | cfssljson -bare join
```
