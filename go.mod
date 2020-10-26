module github.com/bwplotka/mdox

go 1.14

require (
	github.com/Kunde21/markdownfmt/v2 v2.0.2
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/go-kit/kit v0.10.0
	github.com/gohugoio/hugo v0.74.3
	github.com/google/go-cmp v0.4.0 // indirect
	github.com/mattn/go-shellwords v1.0.10
	github.com/mitchellh/mapstructure v1.2.2 // indirect
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/yuin/goldmark v1.2.1
	golang.org/x/net v0.0.0-20200602114024-627f9648deb9 // indirect
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace github.com/Kunde21/markdownfmt/v2 => github.com/bwplotka/markdownfmt/v2 v2.0.0-20201027235426-cd85d2653c78
