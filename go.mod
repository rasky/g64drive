module github.com/rasky/g64drive

go 1.16

require (
	github.com/c2h5oh/datasize v0.0.0-20171227191756-4eba002a5eae
	github.com/mitchellh/go-wordwrap v1.0.0
	github.com/pkg/errors v0.8.1 // indirect
	github.com/schollz/progressbar/v2 v2.13.2
	github.com/smartystreets/goconvey v1.7.2 // indirect
	github.com/spf13/cobra v0.0.5
	github.com/zhuyie/golzf v0.0.0-20161112031142-8387b0307ade
	github.com/ziutek/ftdi v0.0.1
	gopkg.in/ini.v1 v1.62.0
	gopkg.in/restruct.v1 v1.0.0-20190323193435-3c2afb705f3c
)

replace github.com/ziutek/ftdi v0.0.1 => github.com/rasky/ftdi v0.0.2-0.20220228015153-5f910994ef70
