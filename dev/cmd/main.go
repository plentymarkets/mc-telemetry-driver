package main

import (
	_ "github.com/plentymarkets/mc-telemetry-driver/pkg/teldrvr"
	"github.com/plentymarkets/mc-telemetry/pkg/telemetry"
)

func main() {
	telemetry.SetDriver("local")
	telemetry.SetTraceDriver("local")

	transaction, err := telemetry.Start("test transaction")
	if err != nil {
		panic(err)
	}

	sID := transaction.SegmentStart("test segment")
	transaction.AddSegmentAttribute(sID, "test attribute", "test value")
	msg := "test message"
	transaction.Info(sID, &msg)
	transaction.SegmentEnd(sID)
	transaction.Done()
}
