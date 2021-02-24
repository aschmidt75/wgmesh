
For testing purposes, generate a 2nd ca and cert

```bash
$ cfssl gencert -initca ca-csr2.json | cfssljson -bare ca2
$ cfssl gencert -ca ca2.pem -ca-key ca2-key.pem join2-csr.json | cfssljson -bare join2
```
