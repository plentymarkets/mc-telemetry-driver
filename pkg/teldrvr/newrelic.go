package teldrvr

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

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
func (nrd NewRelicDriver) Start(name string) (telemetry.Transaction, error) {
	transactionStart := nrd.NewRelicApp.StartTransaction(name)

	if transactionStart == nil {
		return nil, errors.New("could not start transaction")
	}

	nrt := NewRelicTransaction{
		transaction: *transactionStart,
	}

	return &nrt, nil
}

// NewRelicTransaction used for new relic transactions
type NewRelicTransaction struct {
	transaction      newrelic.Transaction
	segmentContainer NewRelicSegmentContainer
	attributes       map[string]any
	trace            string
}

// NewRelicSegmentContainer used for segment handling
type NewRelicSegmentContainer struct {
	segments   []newrelic.Segment
	attributes map[string]map[string]any
	mutex      sync.RWMutex
}

// AddTransactionAttribute adds an attribute to the transaction
// - Not thread safe -
func (nrt *NewRelicTransaction) AddTransactionAttribute(key string, value any) error {
	val, ok := nrt.attributes[key]
	if ok {
		return fmt.Errorf("attribute '%s' already set with value '%v'", key, val)
	}

	nrt.transaction.AddAttribute(key, value)

	nrt.attributes[key] = value

	return nil
}

// SegmentStart starts a segment in new relic and keeps track of all opened segments
func (nrt *NewRelicTransaction) SegmentStart(name string) error {
	segment := nrt.transaction.StartSegment(name)

	nrt.segments = append(nrt.segments, *segment)
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (nrt *NewRelicTransaction) SegmentEnd() error {
	i := len(nrt.segmentContainer.segments) - 1

	if i < 0 {
		return errors.New("Error trying to end segment. No open segment left")
	}

	nrt.segmentContainer.segments[i].End()

	nSegment := make([]newrelic.Segment, i)

	copy(nSegment, nrt.segmentContainer.segments[:i])

	nrt.segmentContainer.segments = nSegment

	return nil
}

// Error logs errors in the transaction
func (nrt *NewRelicTransaction) Error(readCloser io.ReadCloser) error {
	// max bytes available for the error message
	errMsg := make([]byte, telemetry.ErrorBytesSize)

	_, err := readCloser.Read(errMsg)
	if err != nil {
		readCloser.Close()
		return errors.New("error while reading err message")
	}
	readCloser.Close()

	errLog := errors.New(string(errMsg))

	nrt.transaction.NoticeError(errLog)

	return nil
}

// Info [NOT IMPLEMENTED]
func (nrt *NewRelicTransaction) Info(readCloser io.ReadCloser) error {
	return nil
}

// Done ends a transaction in new relic
func (nrt *NewRelicTransaction) Done() error {
	nrt.transaction.End()

	return nil
}

// CreateTrace creates a trace for the transaction
func (nrt *NewRelicTransaction) CreateTrace() (string, error) {
	header := http.Header{}
	nrt.transaction.InsertDistributedTraceHeaders(header)
	nrt.trace = header.Get(newrelic.DistributedTraceNewRelicHeader)
	return nrt.trace, nil
}

// SetTrace sets a trace for the transaction
func (nrt *NewRelicTransaction) SetTrace(trace string) error {
	header := http.Header{}
	header.Set(newrelic.DistributedTraceNewRelicHeader, trace)
	nrt.transaction.AcceptDistributedTraceHeaders(newrelic.TransportQueue, header)

	return nil
}

// Trace returns the current ttrace for the transaction
func (nrt *NewRelicTransaction) Trace() (string, error) {
	return nrt.trace, nil
}
