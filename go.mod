module github.com/aschmidt75/wgmesh

go 1.15

replace github.com/aschmidt75/wgmesh/cmd => ./cmd

require (
	github.com/aschmidt75/go-wg-wrapper v0.1.0
	github.com/hashicorp/memberlist v0.2.2
	github.com/hashicorp/serf v0.9.5
	github.com/sirupsen/logrus v1.7.0
)
