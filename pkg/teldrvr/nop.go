package teldrvr

import (
	"io"

	"github.com/google/uuid"
	"github.com/plentymarkets/mc-telemetry/pkg/telemetry"
)

/** DRIVER NAME **/
const nopDriver = "nopDriver"

func init() {
	driver := NopDriver{}

	telemetry.RegisterDriver(nopDriver, driver)
}

// nopDriver holds all information the driver needs for telemetry
type NopDriver struct{}

// InitializeTransaction starts a transaction
func (d NopDriver) InitializeTransaction(name string) (telemetry.Transaction, error) {
	transaction := newNopTransaction(name)
	return transaction, nil
}

// NopSegmentContainer used for segment handling
type NopSegmentContainer struct {
}

// NopTransaction used for nop transactions
type NopTransaction struct {
	transaction      string
	segmentContainer NopSegmentContainer
	attributes       map[string]any
	trace            string
	processID        string
}

func newNopTransaction(name string) *NopTransaction {
	t := NopTransaction{}
	return &t
}

// Start no operation
func (t *NopTransaction) Start(name string) {}

// AddTransactionAttribute adds an attribute to the transaction
// - Not thread safe -
func (t *NopTransaction) AddTransactionAttribute(key string, value any) error {
	return nil
}

// SegmentStart starts a nop segment and keeps track of all opened segments
func (t *NopTransaction) SegmentStart(segmentID string, name string) error {
	return nil
}

// AddSegmentAttribute adds an attribute to the currently open segment
// - Thread safe -
func (t *NopTransaction) AddSegmentAttribute(segmentID string, key string, value any) error {
	return nil
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (t *NopTransaction) SegmentEnd(segmentID string) error {
	return nil
}

// Error logs errors in the transaction/segment
func (t *NopTransaction) Error(segmentID string, readCloser io.ReadCloser) error {
	return nil
}

// Info logs information in the transaction
func (t *NopTransaction) Info(segmentID string, readCloser io.ReadCloser) error {
	return nil
}

// Done ends the transaction
func (t *NopTransaction) Done() error {
	return nil
}

// CreateTrace creates a trace for the transaction
func (t *NopTransaction) CreateTrace() (string, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return newUUID.String(), nil
}

// SetTrace sets a trace for the transaction
func (t *NopTransaction) SetTrace(trace string) error {
	t.trace = trace

	return nil
}

// Trace returns the current trace for the transaction
func (t *NopTransaction) Trace() (string, error) {
	return t.trace, nil
}

// TraceID returns the current trace for the transaction, this is the same as trace for every instance but apm
func (t *NopTransaction) TraceID() (string, error) {
	return t.trace, nil
}

// SetTraceID sets a trace for the transaction
func (t *NopTransaction) SetTraceID(traceID string) error {
	return nil
}

// CreateProcessID creates a ProcessID for the transaction
func (t *NopTransaction) CreateProcessID() (string, error) {
	return "", nil
}

// SetProcessID sets a ProcessID for the transaction
func (t *NopTransaction) SetProcessID(processID string) error {
	return nil
}

// ProcessID returns the current ProcessID for the transaction
func (t *NopTransaction) ProcessID() (string, error) {
	return "", nil
}

// Erase any memory the transaction allocated
func (t *NopTransaction) Erase() {
}
