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

	"github.com/google/uuid"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/plentymarkets/mc-telemetry/pkg/telemetry"
)

/** DRIVER NAME **/
const newrelicDriver = "newrelicAPM"

func init() {
	cfg, err := GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	if !strings.Contains(cfg.GetString("telemetry.driver"), newrelicDriver) {
		return
	}

	newRelicApplication, err := newrelic.NewApplication(
		newrelic.ConfigAppName(cfg.GetString("telemetry.app")),
		newrelic.ConfigLicense(cfg.GetString("telemetry.newrelic.licenceKey")),
		newrelic.ConfigAppLogForwardingEnabled(true),
	)
	if err != nil {
		log.Fatalf("newrelic app could not be created, error: %s", err.Error())
	}

	driver := NewRelicAPMDriver{
		NewRelicApp: newRelicApplication,
	}

	telemetry.RegisterDriver(newrelicDriver, driver)
}

// NewRelicAPMDriver holds all information the driver needs for telemetry
type NewRelicAPMDriver struct {
	NewRelicApp *newrelic.Application
}

// InitializeTransaction starts a transaction
func (d NewRelicAPMDriver) InitializeTransaction(name string) (telemetry.Transaction, error) {
	transactionStart := d.NewRelicApp.StartTransaction(name)

	if transactionStart == nil {
		return nil, errors.New("could not start transaction")
	}

	transaction := newAPMTransaction(transactionStart)

	return transaction, nil
}

// NewRelicSegmentContainer used for segment handling
type NewRelicSegmentContainer struct {
	segments   map[string]*newrelic.Segment
	attributes map[string]map[string]any
	mutex      sync.RWMutex
}

// APMTransaction used for new relic transactions
type APMTransaction struct {
	transaction            *newrelic.Transaction
	segmentContainer       NewRelicSegmentContainer
	attributes             map[string]any
	trace                  string
	traceID                string
	processID              string
	segmentsStartWasLogged map[string]struct{}
}

func newAPMTransaction(transaction *newrelic.Transaction) *APMTransaction {
	t := APMTransaction{
		transaction: transaction,
		attributes:  make(map[string]any),
	}
	t.segmentContainer.segments = make(map[string]*newrelic.Segment)
	t.segmentContainer.attributes = make(map[string]map[string]any)
	return &t
}

// Start no operation. This is only added to satisfy the interface
func (t *APMTransaction) Start(name string) {}

// AddTransactionAttribute adds an attribute to the transaction
// - Not thread safe -
func (t *APMTransaction) AddTransactionAttribute(key string, value any) error {
	val, ok := t.attributes[key]
	if ok {
		return fmt.Errorf("attribute '%s' already set with value '%v'", key, val)
	}

	t.transaction.AddAttribute(key, value)
	t.attributes[key] = value

	return nil
}

// SegmentStart starts a segment in new relic and keeps track of all opened segments
func (t *APMTransaction) SegmentStart(segmentID string, name string) error {
	t.segmentContainer.mutex.Lock()
	defer t.segmentContainer.mutex.Unlock()
	segment := t.transaction.StartSegment(name)

	// Failsafe for segments if for some reason they were not initialized
	if t.segmentContainer.segments == nil {
		t.segmentContainer.segments = make(map[string]*newrelic.Segment)
	}

	t.segmentContainer.segments[segmentID] = segment

	return nil
}

// AddSegmentAttribute adds an attribute to the currently open segment
// - Thread safe -
func (t *APMTransaction) AddSegmentAttribute(segmentID string, key string, value any) error {
	t.segmentContainer.mutex.Lock()
	defer t.segmentContainer.mutex.Unlock()

	segment, segmentExist := t.segmentContainer.segments[segmentID]
	if !segmentExist {
		return fmt.Errorf("can not add attribute to not existing segment. SegmentID: %s | Key: %s | Value: %s", segmentID, key, value)
	}

	if t.segmentContainer.attributes[segmentID] == nil {
		t.segmentContainer.attributes[segmentID] = make(map[string]any)
	}

	attribute, attributeExist := t.segmentContainer.attributes[segmentID][key]
	if attributeExist {
		return fmt.Errorf("segment attribute already exist. Segment: %s | SegmentID: %s | Key: %s | Already set value: %v", segment.Name, segmentID, key, attribute)
	}

	t.segmentContainer.attributes[segmentID][key] = value

	segment.AddAttribute(key, value)

	return nil
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (t *APMTransaction) SegmentEnd(segmentID string) error {
	t.segmentContainer.mutex.Lock()
	defer t.segmentContainer.mutex.Unlock()
	segment, ok := t.segmentContainer.segments[segmentID]
	if !ok {
		return fmt.Errorf("Error trying to end segment. Segment is not open. SegmentID: %s", segmentID)
	}

	segment.End()

	delete(t.segmentContainer.segments, segmentID)
	delete(t.segmentContainer.attributes, segmentID)

	return nil
}

// Error logs errors in the transaction
func (t *APMTransaction) Error(_ string, readCloser io.ReadCloser) error {
	// max bytes available for the error message
	errMsg := make([]byte, telemetry.ErrorBytesSize)
	defer func() {
		closeErr := readCloser.Close()
		if closeErr != nil {
			log.Printf("Telemetry driver newRelicAPM could not close reader while logging Info. Potential resource leak!")
		}
	}()

	bytesRead, err := readCloser.Read(errMsg)
	if err != nil {
		return errors.New("error while reading err message")
	}

	errLog := errors.New(string(errMsg[:bytesRead]))

	t.transaction.NoticeError(errLog)

	return nil
}

// Info [NOT IMPLEMENTED]
func (t *APMTransaction) Info(_ string, readCloser io.ReadCloser) error {
	return nil
}

// Debug [NOT IMPLEMENTED]
func (t *APMTransaction) Debug(_ string, readCloser io.ReadCloser) error {
	// TODO - Will be implemented as in Error()
	return nil
}

// Done ends a transaction in new relic
func (t *APMTransaction) Done() error {
	t.transaction.End()

	return nil
}

// CreateTrace creates a trace for the transaction
func (t *APMTransaction) CreateTrace() (string, error) {
	header := http.Header{}
	t.transaction.InsertDistributedTraceHeaders(header)
	t.trace = header.Get(newrelic.DistributedTraceNewRelicHeader)
	return t.trace, nil
}

// SetTrace sets a trace for the transaction
func (t *APMTransaction) SetTrace(trace string) error {
	header := http.Header{}
	header.Set(newrelic.DistributedTraceNewRelicHeader, trace)
	t.transaction.AcceptDistributedTraceHeaders(newrelic.TransportQueue, header)
	t.trace = header.Get(newrelic.DistributedTraceNewRelicHeader)
	t.traceID = t.transaction.GetTraceMetadata().TraceID

	return nil
}

// Trace returns the current trace for the transaction
func (t *APMTransaction) Trace() (string, error) {
	return t.trace, nil
}

// TraceID returns the current traceID for the transaction
func (t *APMTransaction) TraceID() (string, error) {
	return t.traceID, nil
}

// SetTraceID sets a trace for the transaction
func (t *APMTransaction) SetTraceID(traceID string) error {
	return nil
}

// CreateProcessID creates a ProcessID for the transaction
func (t *APMTransaction) CreateProcessID() (string, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return newUUID.String(), nil
}

// SetProcessID sets a ProcessID for the transaction
func (t *APMTransaction) SetProcessID(processID string) error {
	t.processID = processID

	return nil
}

// ProcessID returns the current ProcessID for the transaction
func (t *APMTransaction) ProcessID() (string, error) {
	return t.processID, nil
}

// Erase any memory the transaction allocated
func (t *APMTransaction) Erase() {
	t.attributes = nil
	t.segmentContainer.segments = nil
	t.segmentContainer.attributes = nil

	// we need to collect the garbage manually here because maps in go do have some problems with the garbage collection
	// the runtime.GC method is used to manually free the memory
	// this problem is already known since 2017
	// https://github.com/golang/go/issues/20135
	runtime.GC()
}
