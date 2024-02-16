package main

import (
	"fmt"

	_ "github.com/plentymarkets/mc-telemetry-driver/pkg/teldrvr"
	"github.com/plentymarkets/mc-telemetry/pkg/telemetry"
)

func main() {
	telemetry.SetDriver("nrZerolog")
	telemetry.SetTraceDriver("nrZerolog")

	transaction, _ := telemetry.Start("test")
	for i := 0; i < 10; i++ {
		go func(i int) {
			name := fmt.Sprintf("test: %d", i)
			s1ID := transaction.SegmentStart(name)
			info := fmt.Sprintf("this is an info text lol: %d", i)
			transaction.Info(s1ID, &info)
			err := fmt.Errorf("this is the first error: %d", i)
			transaction.Error(s1ID, &err)
			transaction.SegmentEnd(s1ID)
		}(i)
	}
	// transaction.AddTransactionAttribute("test", "geil")
	// transaction.Done()

	for {
	}
}
