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
	segments   map[string]newrelic.Segment
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
func (nrt *NewRelicTransaction) SegmentStart(segmentID string, name string) error {
	nrt.segmentContainer.mutex.Lock()
	defer nrt.segmentContainer.mutex.Unlock()
	segment := nrt.transaction.StartSegment(name)

	if nrt.segmentContainer.segments == nil {
		nrt.segmentContainer.segments = make(map[string]newrelic.Segment)
	}

	nrt.segmentContainer.segments[segmentID] = *segment

	return nil
}

// AddSegmentAttribute adds an attribute to the currently open segment
// - Thread safe -
func (nrt *NewRelicTransaction) AddSegmentAttribute(segmentID string, key string, value any) error {
	nrt.segmentContainer.mutex.Lock()
	defer nrt.segmentContainer.mutex.Unlock()

	segment, segmentExist := nrt.segmentContainer.segments[segmentID]
	if !segmentExist {
		return fmt.Errorf("can not add attribute to not existing segment.\nSegmentID: %s\nKey: %s\nValue: %s", segmentID, key, value)
	}

	if nrt.segmentContainer.attributes == nil {
		nrt.segmentContainer.attributes = make(map[string]map[string]any)
	}

	if nrt.segmentContainer.attributes[segmentID] == nil {
		nrt.segmentContainer.attributes[segmentID] = make(map[string]any)
	}

	attribute, attributeExist := nrt.segmentContainer.attributes[segmentID][key]
	if attributeExist {
		return fmt.Errorf("segment attribute already exist.\nSegment: %s\nSegmentID: %s\nKey: %s\nAlready set value: %v", segment.Name, segmentID, key, attribute)
	}

	nrt.segmentContainer.attributes[segmentID][key] = value

	segment.AddAttribute(key, value)

	return nil
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (nrt *NewRelicTransaction) SegmentEnd(segmentID string) error {
	nrt.segmentContainer.mutex.Lock()
	defer nrt.segmentContainer.mutex.Unlock()
	segment, ok := nrt.segmentContainer.segments[segmentID]
	if !ok {
		return fmt.Errorf("Error trying to end segment. Segment is not open.\nSegmentID: %s", segmentID)
	}

	segment.End()

	delete(nrt.segmentContainer.segments, segmentID)
	delete(nrt.segmentContainer.attributes, segmentID)

	return nil
}

// Error logs errors in the transaction
func (nrt *NewRelicTransaction) Error(_ string, readCloser io.ReadCloser) error {
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
func (nrt *NewRelicTransaction) Info(_ string, readCloser io.ReadCloser) error {
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

// Erase any memory the transaction allocated
func (nrt *NewRelicTransaction) Erase() {
	nrt.attributes = nil
	nrt.segmentContainer.segments = nil
	nrt.segmentContainer.attributes = nil

	// we need to collect the garbage manually here because maps in go do have some problems with the garbage collection
	// the runtime.GC method is used to manually free the memory
	// this problem is already known since 2017
	// https://github.com/golang/go/issues/20135
	runtime.GC()
}
