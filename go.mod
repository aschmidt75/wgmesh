module github.com/aschmidt75/wgmesh

go 1.15

replace github.com/aschmidt75/wgmesh/cmd => ./cmd

require (
	github.com/armon/go-metrics v0.3.6 // indirect
	github.com/aschmidt75/go-wg-wrapper v0.1.1-0.20210206151906-a540c071fbf8
	github.com/golang/protobuf v1.4.3
	github.com/google/btree v1.0.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.0 // indirect
	github.com/hashicorp/go-msgpack v1.1.5 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/memberlist v0.2.2
	github.com/hashicorp/serf v0.9.5
	github.com/mdlayher/netlink v1.2.1 // indirect
	github.com/miekg/dns v1.1.38 // indirect
	github.com/sirupsen/logrus v1.7.0
	go.opencensus.io v0.22.6
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/net v0.0.0-20210119194325-5f4716e94777 // indirect
	golang.org/x/sys v0.0.0-20210124154548-22da62e12c0c // indirect
	golang.org/x/text v0.3.5 // indirect
	golang.zx2c4.com/wireguard v0.0.20201118 // indirect
	google.golang.org/genproto v0.0.0-20210201184850-646a494a81ea // indirect
	google.golang.org/grpc v1.35.0
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.1.0 // indirect
	google.golang.org/protobuf v1.25.0
	gortc.io/stun v1.23.0
)
