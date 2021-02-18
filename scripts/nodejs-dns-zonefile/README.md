# nodejs-dns-zonefile

This directory includes a small NodeJS script to transform the output of the mesh member list (using `-memberlist-file`)
into a DNS zone file compatible DNS record set. It reads the memberlist file from STDIN and writes zone records to STDOUT,
based upon a template file (see `sample-template.dns`). 

Wgmesh updates the memberlist every time a change event occurs within the mesh. It includes a timestamp which can be used as a serial within the zone file (see template).

```bash
$ node --version
v10.19.0
$ npm install
(...)

$ cat /var/log/memberlist.json | node index.js >records.dns
```

As an example, [coredns](https://coredns.io) is able to serve this using a sample configuration

```bash
$ cat >coredns.cfg <<EOF

samplemesh.local:8053 {
	bind 127.0.0.1

	auto {
		directory /path/to/zonefiles (.*).cfg {1}
		reload 2s
	}

	log
}
EOF

$ cat /var/log/memberlist.json | node index.js sample-template.dns >/path/to/output/of/this/script.inc
```

Then
```bash
$ coredns -conf coredns.conf
$ dig -p 8053 @127.0.0.1 node1.samplemesh.local +short
(...)
```