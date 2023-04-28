package teldrvr

import (
	"errors"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/plentymarkets/mc-telemetry/pkg/telemetry"
)

/** DRIVER NAME **/
const newrelicDriver = "newrelic"

func init() {
	cfg, err := GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	if !strings.Contains(cfg.GetString("telemetry.driver"), newrelicDriver) {
		return
	}

	nra, err := newrelic.NewApplication(
		newrelic.ConfigAppName(cfg.GetString("telemetry.app")),
		newrelic.ConfigLicense(cfg.GetString("telemetry.newrelic.licenceKey")),
		newrelic.ConfigAppLogForwardingEnabled(true),
	)
	if err != nil {
		log.Fatalf("newrelic app could not be created, error: %s", err.Error())
	}

	nrd := NewRelicDriver{
		NewRelicApp: nra,
	}

	telemetry.RegisterDriver(newrelicDriver, nrd)
}

// NewRelicDriver holds all information the driver needs for telemetry
type NewRelicDriver struct {
	NewRelicApp *newrelic.Application
}

// Start starts a transaction
func (nrd NewRelicDriver) Start(name string) telemetry.Transaction {
	nrt := NewRelicTransaction{
		transaction: *nrd.NewRelicApp.StartTransaction(name),
	}

	return &nrt
}

// NewRelicTransaction used for new relic transactions
type NewRelicTransaction struct {
	transaction newrelic.Transaction
	segments    []newrelic.Segment
	attributes  map[string]any
	trace       string
}

// AddAttribute adds an attribute to the transaction
func (nrt *NewRelicTransaction) AddAttribute(key string, value any) {
	val, ok := nrt.attributes[key]
	if ok {
		log.Printf("attribute '%s' already set with value '%v'", key, val)
		return
	}

	nrt.transaction.AddAttribute(key, value)

	nrt.attributes[key] = value
}

// SegmentStart starts a segment in new relic and keeps track of all opened segments
func (nrt *NewRelicTransaction) SegmentStart(name string) {
	segment := nrt.transaction.StartSegment(name)

	nrt.segments = append(nrt.segments, *segment)
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (nrt *NewRelicTransaction) SegmentEnd() {
	i := len(nrt.segments) - 1

	nrt.segments[i].End()

	nSegment := make([]newrelic.Segment, i)

	copy(nSegment, nrt.segments[:i])

	nrt.segments = nSegment
}

// Error logs errors in the transaction
func (nrt *NewRelicTransaction) Error(readCloser io.ReadCloser) {
	// max bytes available for the error message
	errMsg := make([]byte, telemetry.ErrorBytesSize)

	_, err := readCloser.Read(errMsg)
	if err != nil {
		readCloser.Close()
		log.Panicln("error while reading err message")
	}
	readCloser.Close()

	errLog := errors.New(string(errMsg))

	nrt.transaction.NoticeError(errLog)
}

// Info [NOT IMPLEMENTED]
func (nrt *NewRelicTransaction) Info(readCloser io.ReadCloser) {
}

// Done ends a transaction in new relic
func (nrt *NewRelicTransaction) Done() {
	nrt.transaction.End()
}

// CreateTrace creates a trace for the transaction
func (nrt *NewRelicTransaction) CreateTrace() string {
	header := http.Header{}
	nrt.transaction.InsertDistributedTraceHeaders(header)
	nrt.trace = header.Get(newrelic.DistributedTraceNewRelicHeader)
	return nrt.trace
}

// SetTrace sets a trace for the transaction
func (nrt *NewRelicTransaction) SetTrace(trace string) {
	header := http.Header{}
	header.Set(newrelic.DistributedTraceNewRelicHeader, trace)
	nrt.transaction.AcceptDistributedTraceHeaders(newrelic.TransportQueue, header)
}

// Trace returns the current ttrace for the transaction
func (nrt *NewRelicTransaction) Trace() string {
	return nrt.trace
}
