module github.com/temporalio/roadrunner-temporal

go 1.15

require (
	github.com/json-iterator/go v1.1.10
	github.com/pkg/errors v0.9.1
	github.com/shirou/gopsutil v2.20.9+incompatible
	github.com/spf13/viper v1.7.1
	github.com/spiral/endure v1.0.0-beta7
	github.com/spiral/goridge/v2 v2.4.5
	github.com/spiral/roadrunner v1.8.3
	github.com/stretchr/testify v1.6.1
	github.com/valyala/tcplisten v0.0.0-20161114210144-ceec8f93295a
	go.uber.org/multierr v1.6.0 // indirect
	golang.org/x/sync v0.0.0-20190911185100-cd5d95a43a6e
)

replace github.com/spiral/endure v1.0.0-beta7 => /home/valery/Projects/opensource/spiral/endure
