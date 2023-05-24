package teldrvr

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"runtime"
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

	nrt.segmentContainer.segments = append(nrt.segmentContainer.segments, *segment)

	return nil
}

// AddSegmentAttribute adds an attribute to the currently open segment
// - Thread safe -
func (nrt *NewRelicTransaction) AddSegmentAttribute(key string, value any) error {
	nrt.segmentContainer.mutex.Lock()
	defer nrt.segmentContainer.mutex.Unlock()

	if len(nrt.segmentContainer.segments) == 0 {
		return fmt.Errorf("can not add attribute to not existing segment. Key: %s Value: %s", key, value)
	}

	if nrt.segmentContainer.attributes == nil {
		nrt.segmentContainer.attributes = make(map[string]map[string]any)
	}

	currentOpenSegment := nrt.segmentContainer.segments[len(nrt.segmentContainer.segments)-1]

	if nrt.segmentContainer.attributes[currentOpenSegment.Name] == nil {
		nrt.segmentContainer.attributes[currentOpenSegment.Name] = make(map[string]any)
	}

	val, ok := nrt.segmentContainer.attributes[currentOpenSegment.Name][key]
	if ok {
		return fmt.Errorf("segment attribute '%s' already set with value '%v'", key, val)
	}

	nrt.segmentContainer.attributes[currentOpenSegment.Name][key] = value

	currentOpenSegment.AddAttribute(key, value)

	return nil
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
	nrt.eraseMemory()

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

// eraseMemory erase any memory the transaction allocated
func (nrt *NewRelicTransaction) eraseMemory() {
	nrt.attributes = nil
	nrt.segmentContainer.segments = nil
	nrt.segmentContainer.attributes = nil

	// we need to collect the garbage manually here because maps in go do have some problems with the garbage collection
	// the runtime.GC method is used to manually free the memory
	// this problem is already known since 2017
	// https://github.com/golang/go/issues/20135
	runtime.GC()
}
