package util

import (
	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"io"
	"strconv"
	"time"

	"github.com/spiral/roadrunner/v2"
)

// WorkerTable renders table with information about rr server workers.
func WorkerTable(writer io.Writer, workers []roadrunner.ProcessState) *tablewriter.Table {
	tw := tablewriter.NewWriter(writer)
	tw.SetHeader([]string{"PID", "Status", "Execs", "Memory", "Created"})
	tw.SetColMinWidth(0, 7)
	tw.SetColMinWidth(1, 9)
	tw.SetColMinWidth(2, 7)
	tw.SetColMinWidth(3, 7)
	tw.SetColMinWidth(4, 18)

	for _, w := range workers {
		tw.Append([]string{
			strconv.Itoa(w.Pid),
			renderStatus(w.Status),
			renderJobs(w.NumJobs),
			humanize.Bytes(w.MemoryUsage),
			renderAlive(time.Unix(0, w.Created)),
		})
	}

	return tw
}

func renderStatus(status string) string {
	switch status {
	case "inactive":
		return color.YellowString("inactive")
	case "ready":
		return color.CyanString("ready")
	case "working":
		return color.GreenString("working")
	case "invalid":
		return color.YellowString("invalid")
	case "stopped":
		return color.RedString("stopped")
	case "errored":
		return color.RedString("errored")
	}

	return status
}

func renderJobs(number int64) string {
	return humanize.Comma(number)
}

func renderAlive(t time.Time) string {
	return humanize.RelTime(t, time.Now(), "ago", "")
}
