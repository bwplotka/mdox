module github.com/bwplotka/mdox

go 1.15

require (
	github.com/Kunde21/markdownfmt/v2 v2.1.0
	github.com/efficientgo/tools/core v0.0.0-20210609125236-d73259166f20
	github.com/efficientgo/tools/extkingpin v0.0.0-20210609125236-d73259166f20
	github.com/go-kit/kit v0.10.0
	github.com/gobwas/glob v0.2.3
	github.com/gocolly/colly/v2 v2.1.0
	github.com/gohugoio/hugo v0.74.3
	github.com/kr/pretty v0.2.1 // indirect
	github.com/mattn/go-shellwords v1.0.10
	github.com/mitchellh/mapstructure v1.2.2 // indirect
	github.com/oklog/run v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/sergi/go-diff v1.0.0
	github.com/yuin/goldmark v1.3.5
	golang.org/x/lint v0.0.0-20200302205851-738671d3881b // indirect
	golang.org/x/net v0.0.0-20200822124328-c89045814202
	golang.org/x/sys v0.0.0-20200615200032-f1bc736245b1 // indirect
	golang.org/x/tools v0.0.0-20201020161133-226fd2f889ca // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776
)

// TODO(bwplotka): Remove when https://github.com/Kunde21/markdownfmt/pull/35 is merged.
replace github.com/Kunde21/markdownfmt/v2 => github.com/bwplotka/markdownfmt/v2 v2.0.0-20210616121647-559e77044d46
