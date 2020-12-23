module github.com/bwplotka/mdox

go 1.14

require (
	github.com/Kunde21/markdownfmt/v2 v2.0.4-0.20201214081534-353201c4cdce
	github.com/alecthomas/units v0.0.0-20190924025748-f65c72e2690d // indirect
	github.com/go-kit/kit v0.10.0
	github.com/gocolly/colly/v2 v2.1.0
	github.com/gohugoio/hugo v0.74.3
	github.com/mattn/go-shellwords v1.0.10
	github.com/mitchellh/mapstructure v1.2.2 // indirect
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/sergi/go-diff v1.0.0
	github.com/stretchr/testify v1.5.1
	github.com/yuin/goldmark v1.2.1
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
)

replace github.com/Kunde21/markdownfmt/v2 => github.com/bwplotka/markdownfmt/v2 v2.0.0-20201223130030-c496cb0bcc88
