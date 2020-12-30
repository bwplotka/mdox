module github.com/bwplotka/mdox

go 1.14

require (
	github.com/Kunde21/markdownfmt/v2 v2.0.4-0.20201214081534-353201c4cdce
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/efficientgo/tools/core v0.0.0-20201228212819-69909db83cda
	github.com/go-kit/kit v0.10.0
	github.com/gocolly/colly/v2 v2.1.0
	github.com/gohugoio/hugo v0.74.3
	github.com/mattn/go-shellwords v1.0.10
	github.com/mitchellh/mapstructure v1.2.2 // indirect
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.3.0
	github.com/prometheus/common v0.7.0
	github.com/sergi/go-diff v1.0.0
	github.com/yuin/goldmark v1.2.1
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace (
	github.com/Kunde21/markdownfmt/v2 => github.com/bwplotka/markdownfmt/v2 v2.0.0-20201225192631-f2e7830d9793
	github.com/efficientgo/tools/core => ../tools/core
)
