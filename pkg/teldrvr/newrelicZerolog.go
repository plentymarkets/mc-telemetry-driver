package teldrvr

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/zerologWriter"
	"github.com/newrelic/go-agent/v3/newrelic"

	"github.com/google/uuid"

	"github.com/plentymarkets/mc-telemetry/pkg/telemetry"
	"github.com/rs/zerolog"
)

/** DRIVER NAME **/
const zerologDriver = "nrZerolog"

const newRelicZerologDebug = "debug"
const newRelicZerologError = "error"
const newRelicZerologInfo = "info"

func init() {
	cfg, err := GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	if !strings.Contains(cfg.GetString("telemetry.driver"), zerologDriver) {
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

	configLogLevel := cfg.GetString("telemetry.logLevel")
	switch configLogLevel {
	case logLevelDebug:
		logLevel = logLevelDebug
		break
	case logLevelError:
		logLevel = logLevelError
		break
	case logLevelInfo:
		logLevel = logLevelInfo
		break
	default:
		log.Println("Got unknown log level from config. Fallback to error level")
		logLevel = logLevelError
	}

	driver := ZeroLogDriver{
		NewRelicApp: newRelicApplication,
	}

	telemetry.RegisterDriver(zerologDriver, driver)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

// ZeroLogDriver holds all information the driver needs for telemetry
type ZeroLogDriver struct {
	NewRelicApp *newrelic.Application
}

// InitializeTransaction starts a transaction
func (d ZeroLogDriver) InitializeTransaction(name string) (telemetry.Transaction, error) {
	writer := zerologWriter.New(os.Stdout, d.NewRelicApp)
	logger := zerolog.New(writer).With().Timestamp().Logger()

	transaction := newZeroLogTransaction(logger)

	return transaction, nil
}

func (t *ZeroLogTransaction) logTrace(msg string) {
	preparedLog := t.transaction.Info()
	if t.trace != "" {
		preparedLog.Str("traceID", t.trace)
	}
	preparedLog.Str("processID", t.processID)

	for key, value := range t.attributes {
		preparedLog.Any(key, value)
	}

	preparedLog.Msg(msg)
}

// ZeroLogSegmentContainer used for segment handling
type ZeroLogSegmentContainer struct {
	segments               map[string]string         // key = segment ID | value = name of the segment
	attributes             map[string]map[string]any // {"segmentID":  {"attributeName": "attribute value"}}
	mutex                  sync.RWMutex
	segmentsStartWasLogged map[string]struct{}
}

// ZeroLogTransaction used for local transactions
type ZeroLogTransaction struct {
	name             string
	transaction      zerolog.Logger
	segmentContainer ZeroLogSegmentContainer
	attributes       map[string]any
	trace            string
	processID        string
}

func newZeroLogTransaction(logger zerolog.Logger) *ZeroLogTransaction {
	t := ZeroLogTransaction{
		transaction: logger,
		attributes:  make(map[string]any),
	}
	t.segmentContainer.segments = make(map[string]string)
	t.segmentContainer.attributes = make(map[string]map[string]any)
	t.segmentContainer.segmentsStartWasLogged = make(map[string]struct{})
	return &t
}

// Start writes the starting message of the transaction
func (t *ZeroLogTransaction) Start(name string) {
	t.name = name
	msg := fmt.Sprintf("Transaction start: %s", name)
	t.logTrace(msg)
}

// AddTransactionAttribute adds an attribute to the transaction
// - Not thread safe -
func (t *ZeroLogTransaction) AddTransactionAttribute(key string, value any) error {
	val, ok := t.attributes[key]
	if ok {
		return fmt.Errorf("transaction attribute '%s' already set with value '%v'", key, val)
	}

	t.attributes[key] = value

	return nil
}

// SegmentStart starts a local segment and keeps track of all opened segments
func (t *ZeroLogTransaction) SegmentStart(segmentID string, name string) error {
	t.segmentContainer.mutex.Lock()
	defer t.segmentContainer.mutex.Unlock()
	t.segmentContainer.segments[segmentID] = name
	if logLevel == logLevelDebug {
		return t.segmentWriteStart(segmentID)
	}

	return nil
}

func (t *ZeroLogTransaction) segmentWriteStart(segmentID string) error {
	if _, ok := t.segmentContainer.segmentsStartWasLogged[segmentID]; ok {
		return nil
	}
	var name string
	ok := false
	if name, ok = t.segmentContainer.segments[segmentID]; !ok {
		return fmt.Errorf("segment name not found for segmentID: %s", segmentID)
	}

	msg := fmt.Sprintf("Segment start: %s", name)
	readCloser := io.NopCloser(strings.NewReader(msg))

	err := t.infoWithAlreadyLockedMutex(segmentID, readCloser)
	if err != nil {
		return err
	}

	t.segmentContainer.segmentsStartWasLogged[segmentID] = struct{}{}

	return nil
}

func (t *ZeroLogTransaction) infoWithAlreadyLockedMutex(segmentID string, readCloser io.ReadCloser) error {
	return t.logMessageWithAlreadyLockedMutex(newRelicZerologInfo, segmentID, readCloser)
}

func (t *ZeroLogTransaction) logMessageWithAlreadyLockedMutex(level string, segmentID string, readCloser io.ReadCloser) error {
	defer func() {
		closeErr := readCloser.Close()
		if closeErr != nil {
			log.Printf("Telemetry driver newRelicZerolog could not close reader while logging Info. Potential resource leak!")
		}
	}()
	// max bytes available for the info message
	msg := make([]byte, telemetry.ErrorBytesSize)

	bytesRead, err := readCloser.Read(msg)
	if err != nil {
		return errors.New("error while reading message")
	}

	logMsg := string(msg[:bytesRead])
	var preparedLog *zerolog.Event

	switch level {
	case newRelicZerologInfo:
		preparedLog = t.transaction.Info()
		break
	case newRelicZerologError:
		preparedLog = t.transaction.Error()
		break
	default:
		return errors.New("unknown log level")
	}

	preparedLog.
		Str("processID", t.processID).
		Str("traceID", t.trace).
		Str("segmentID", segmentID).
		Str("action", t.segmentContainer.segments[segmentID])

	for key, value := range t.segmentContainer.attributes[segmentID] {
		preparedLog.Any(key, value)
	}

	preparedLog.Msg(logMsg)

	return nil
}

// AddSegmentAttribute adds an attribute to the currently open segment
// - Thread safe -
func (t *ZeroLogTransaction) AddSegmentAttribute(segmentID string, key string, value any) error {
	t.segmentContainer.mutex.Lock()
	defer t.segmentContainer.mutex.Unlock()

	segmentName, segmentExist := t.segmentContainer.segments[segmentID]
	if !segmentExist {
		return fmt.Errorf("can not add attribute to not existing segment. SegmentID: %s | Key: %s | Value: %s", segmentID, key, value)
	}

	if t.segmentContainer.attributes[segmentID] == nil {
		t.segmentContainer.attributes[segmentID] = make(map[string]any)
	}

	attribute, attributeExist := t.segmentContainer.attributes[segmentID][key]
	if attributeExist {
		return fmt.Errorf("segment attribute already exist. Segment: %s | SegmentID: %s | Key: %s | Already set value: %v", segmentName, segmentID, key, attribute)
	}

	t.segmentContainer.attributes[segmentID][key] = value

	return nil
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (t *ZeroLogTransaction) SegmentEnd(segmentID string) error {
	t.segmentContainer.mutex.Lock()
	defer t.segmentContainer.mutex.Unlock()
	_, ok := t.segmentContainer.segments[segmentID]
	if !ok {
		return fmt.Errorf("Error trying to end segment. Segment is not open. SegmentID: %s", segmentID)
	}

	err := t.segmentWriteEnd(segmentID)
	if err != nil {
		return err
	}

	return nil
}

func (t *ZeroLogTransaction) segmentWriteEnd(segmentID string) error {
	if _, ok := t.segmentContainer.segmentsStartWasLogged[segmentID]; !ok {
		delete(t.segmentContainer.segments, segmentID)
		delete(t.segmentContainer.attributes, segmentID)
		return nil
	}

	name, ok := t.segmentContainer.segments[segmentID]
	if !ok {
		return fmt.Errorf("Error trying to end segment. Segment is not open.\nSegmentID: %s", segmentID)
	}

	msg := fmt.Sprintf("Segment end: %s", name)
	readCloser := io.NopCloser(strings.NewReader(msg))
	// implement using our info method
	err := t.infoWithAlreadyLockedMutex(segmentID, readCloser)

	if err != nil {
		return err
	}

	delete(t.segmentContainer.segments, segmentID)
	delete(t.segmentContainer.attributes, segmentID)
	delete(t.segmentContainer.segmentsStartWasLogged, segmentID)
	return nil
}

// Error logs errors in the transaction
func (t *ZeroLogTransaction) Error(segmentID string, readCloser io.ReadCloser) error {
	return t.logMessage(newRelicZerologError, segmentID, readCloser)
}

func (t *ZeroLogTransaction) logMessage(level string, segmentID string, readCloser io.ReadCloser) error {
	t.segmentContainer.mutex.Lock()
	defer func() {
		t.segmentContainer.mutex.Unlock()
		closeErr := readCloser.Close()
		if closeErr != nil {
			log.Printf("Telemetry driver newRelicZerolog could not close reader while logging Info. Potential resource leak!")
		}
	}()
	t.segmentWriteStart(segmentID)
	// max bytes available for the info message
	msg := make([]byte, telemetry.ErrorBytesSize)

	bytesRead, err := readCloser.Read(msg)
	if err != nil {
		return errors.New("error while reading message")
	}

	logMsg := string(msg[:bytesRead])
	var preparedLog *zerolog.Event

	switch level {
	case newRelicZerologInfo:
		preparedLog = t.transaction.Info()
		break
	case newRelicZerologError:
		preparedLog = t.transaction.Error()
		break
	default:
		return errors.New("unknown log level")
	}

	preparedLog.
		Str("processID", t.processID).
		Str("traceID", t.trace).
		Str("segmentID", segmentID).
		Str("action", t.segmentContainer.segments[segmentID])

	for key, value := range t.segmentContainer.attributes[segmentID] {
		preparedLog.Any(key, value)
	}

	preparedLog.Msg(logMsg)

	return nil
}

// Info logs errors in the transaction
func (t *ZeroLogTransaction) Info(segmentID string, readCloser io.ReadCloser) error {
	if logLevel == logLevelError {
		return nil
	}
	return t.logMessage(newRelicZerologInfo, segmentID, readCloser)
}

// Done ends the transaction
func (t *ZeroLogTransaction) Done() error {
	msg := fmt.Sprintf("Transaction end: %s", t.name)
	t.logTrace(msg)
	t.Erase()

	return nil
}

// CreateTrace creates a trace for the transaction
func (t *ZeroLogTransaction) CreateTrace() (string, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return newUUID.String(), nil
}

// SetTrace sets a trace for the transaction
func (t *ZeroLogTransaction) SetTrace(trace string) error {
	t.trace = trace

	return nil
}

// Trace returns the current ttrace for the transaction
func (t *ZeroLogTransaction) Trace() (string, error) {
	return t.trace, nil
}

// TraceID returns the current trace for the transaction, this is the same as trace for every instance but apm
func (t *ZeroLogTransaction) TraceID() (string, error) {
	return t.trace, nil
}

// SetTraceID sets a trace for the transaction
func (t *ZeroLogTransaction) SetTraceID(traceID string) error {
	t.trace = traceID
	return nil
}

// CreateProcessID creates a ProcessID for the transaction
func (t *ZeroLogTransaction) CreateProcessID() (string, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return newUUID.String(), nil
}

// SetProcessID sets a ProcessID for the transaction
func (t *ZeroLogTransaction) SetProcessID(processID string) error {
	t.processID = processID

	return nil
}

// ProcessID returns the current ProcessID for the transaction
func (t *ZeroLogTransaction) ProcessID() (string, error) {
	return t.processID, nil
}

// Erase any memory the transaction allocated
func (t *ZeroLogTransaction) Erase() {
	t.attributes = nil
	t.segmentContainer.segments = nil
	t.segmentContainer.attributes = nil

	// we need to collect the garbage manually here because maps in go do have some problems with the garbage collection
	// the runtime.GC method is used to manually free the memory
	// this problem is already known since 2017
	// https://github.com/golang/go/issues/20135
	runtime.GC()
}
