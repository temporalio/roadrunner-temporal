module github.com/temporalio/roadrunner-temporal

go 1.15

require (
	github.com/fatih/color v1.7.0
	github.com/goccy/go-graphviz v0.0.8 // indirect
	github.com/json-iterator/go v1.1.10
	github.com/prometheus/common v0.4.0
	github.com/spf13/cobra v1.1.0
	github.com/spiral/endure v1.0.0-beta12
	github.com/spiral/errors v1.0.1
	github.com/spiral/roadrunner/v2 v2.0.0-alpha14
	go.temporal.io/api v1.0.0
	go.temporal.io/sdk v1.1.0
	go.uber.org/zap v1.16.0
	golang.org/x/image v0.0.0-20200927104501-e162460cd6b5 // indirect
)

replace github.com/spiral/roadrunner/v2 v2.0.0-alpha14 => /home/valery/Projects/opensource/spiral/roadrunner
