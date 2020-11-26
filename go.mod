module github.com/temporalio/roadrunner-temporal

go 1.15

require (
	github.com/StackExchange/wmi v0.0.0-20190523213315-cbe66965904d // indirect
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.10.0
	github.com/go-ole/go-ole v1.2.4 // indirect
	github.com/json-iterator/go v1.1.10
	github.com/mattn/go-runewidth v0.0.9
	github.com/olekukonko/tablewriter v0.0.4
	github.com/spf13/cobra v1.1.0
	github.com/spiral/endure v1.0.0-beta19
	github.com/spiral/errors v1.0.4
	github.com/spiral/goridge/v2 v2.4.6
	github.com/spiral/roadrunner/v2 v2.0.0-alpha20
	github.com/vbauerster/mpb/v5 v5.3.0
	go.temporal.io/api v1.2.0
	go.temporal.io/sdk v1.1.0
	go.uber.org/zap v1.16.0
)

replace go.temporal.io/sdk v1.1.0 => ../sdk-go
