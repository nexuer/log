module github.com/nexuer/log/benchmarks

go 1.25.12

replace github.com/nexuer/log => ../

require (
	github.com/nexuer/log v0.0.0
	github.com/phuslu/log v1.0.127
	github.com/rs/zerolog v1.35.1
	github.com/sirupsen/logrus v1.9.4
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
)

require (
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	golang.org/x/sys v0.29.0 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
)
