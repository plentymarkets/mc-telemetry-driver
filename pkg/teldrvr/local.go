package teldrvr

import (
	"errors"
	"fmt"
	"io"
	"log"
	"runtime"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/plentymarkets/mc-telemetry/pkg/telemetry"
)

/** DRIVER NAME **/
const localDriver = "local"

func init() {
	cfg, err := GetConfig()
	if err != nil {
		log.Fatal(err)
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

	driver := LocalDriver{}

	telemetry.RegisterDriver(localDriver, driver)
}

// LocalDriver holds all information the driver needs for telemetry
type LocalDriver struct{}

// InitializeTransaction starts a transaction
func (d LocalDriver) InitializeTransaction(name string) (telemetry.Transaction, error) {
	transaction := newLocalTransaction(name)
	return transaction, nil
}

// LocalSegmentContainer used for segment handling
type LocalSegmentContainer struct {
	segments               map[string]string
	attributes             map[string]map[string]any
	mutex                  sync.RWMutex
	segmentsStartWasLogged map[string]struct{}
}

// LocalTransaction used for local transactions
type LocalTransaction struct {
	transaction      string
	segmentContainer LocalSegmentContainer
	attributes       map[string]any
	trace            string
	processID        string
}

func newLocalTransaction(name string) *LocalTransaction {
	t := LocalTransaction{
		transaction: name,
		attributes:  make(map[string]any),
	}
	t.segmentContainer.segments = make(map[string]string)
	t.segmentContainer.attributes = make(map[string]map[string]any)
	t.segmentContainer.segmentsStartWasLogged = make(map[string]struct{})
	return &t
}

// Start writes the start message for the transaction
func (t *LocalTransaction) Start(name string) {
	if t.trace != "" {
		log.Printf("Transaction %s start: %s \n", t.trace, name)
	}
	log.Printf("Transaction processID %s start: %s \n", t.processID, name)
}

// AddTransactionAttribute adds an attribute to the transaction
// - Not thread safe -
func (t *LocalTransaction) AddTransactionAttribute(key string, value any) error {
	val, ok := t.attributes[key]
	if ok {
		return fmt.Errorf("transaction attribute '%s' already set with value '%v'", key, val)
	}

	t.attributes[key] = value

	return nil
}

// SegmentStart starts a local segment and keeps track of all opened segments
func (t *LocalTransaction) SegmentStart(segmentID string, name string) error {
	var err error
	t.segmentContainer.mutex.Lock()
	defer t.segmentContainer.mutex.Unlock()
	t.segmentContainer.segments[segmentID] = name
	if logLevel == logLevelDebug {
		err = t.segmentWriteStart(segmentID)
	}

	if err != nil {
		return err
	}

	return nil
}

func (t *LocalTransaction) segmentWriteStart(segmentID string) error {
	if _, ok := t.segmentContainer.segmentsStartWasLogged[segmentID]; ok {
		return nil
	}
	var name string
	ok := false
	if name, ok = t.segmentContainer.segments[segmentID]; !ok {
		return fmt.Errorf("segment name not found for segmentID: %s", segmentID)
	}
	log.Printf("Segment start[%s]: %s \n", segmentID, name)
	t.segmentContainer.segmentsStartWasLogged[segmentID] = struct{}{}

	return nil
}

// AddSegmentAttribute adds an attribute to the currently open segment
// - Thread safe -
func (t *LocalTransaction) AddSegmentAttribute(segmentID string, key string, value any) error {
	t.segmentContainer.mutex.Lock()
	defer t.segmentContainer.mutex.Unlock()

	segmentName, segmentExist := t.segmentContainer.segments[segmentID]
	if !segmentExist {
		return fmt.Errorf("can not add attribute to not existing segment.\nSegmentID: %s\nKey: %s\nValue: %s", segmentID, key, value)
	}

	if t.segmentContainer.attributes[segmentID] == nil {
		t.segmentContainer.attributes[segmentID] = make(map[string]any)
	}

	attribute, attributeExist := t.segmentContainer.attributes[segmentID][key]
	if attributeExist {
		return fmt.Errorf("segment attribute already exist.\nSegment: %s\nSegmentID: %s\nKey: %s\nAlready set value: %v", segmentName, segmentID, key, attribute)
	}

	t.segmentContainer.attributes[segmentID][key] = value

	return nil
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (t *LocalTransaction) SegmentEnd(segmentID string) error {
	t.segmentContainer.mutex.Lock()
	defer t.segmentContainer.mutex.Unlock()
	_, ok := t.segmentContainer.segments[segmentID]
	if !ok {
		return fmt.Errorf("Error trying to end segment. Segment is not open.\nSegmentID: %s", segmentID)
	}

	t.segmentWriteEnd(segmentID)

	return nil
}

func (t *LocalTransaction) segmentWriteEnd(segmentID string) error {
	if _, ok := t.segmentContainer.segmentsStartWasLogged[segmentID]; !ok {
		delete(t.segmentContainer.segments, segmentID)
		delete(t.segmentContainer.attributes, segmentID)
		return nil
	}

	name, ok := t.segmentContainer.segments[segmentID]
	if !ok {
		return fmt.Errorf("Error trying to end segment. Segment is not open.\nSegmentID: %s", segmentID)
	}
	// todo add the attributes
	log.Printf("Segment end[%s]: %s\n", segmentID, name)

	delete(t.segmentContainer.segments, segmentID)
	delete(t.segmentContainer.attributes, segmentID)
	delete(t.segmentContainer.segmentsStartWasLogged, segmentID)

	return nil
}

// Error logs errors in the transaction/segment
func (t *LocalTransaction) Error(segmentID string, readCloser io.ReadCloser) error {
	t.segmentContainer.mutex.Lock()
	defer func() {
		t.segmentContainer.mutex.Unlock()
		closeErr := readCloser.Close()
		if closeErr != nil {
			log.Printf("Telemetry driver local could not close reader while logging Info. Potential resource leak!")
		}
	}()
	t.segmentWriteStart(segmentID)
	// max bytes available for the error message
	errMsg := make([]byte, telemetry.ErrorBytesSize)

	bytesRead, err := readCloser.Read(errMsg)
	if err != nil {
		return errors.New("error while reading err message")
	}

	errLog := string(errMsg[:bytesRead])

	inSegment := false
	if len(segmentID) > 0 {
		_, ok := t.segmentContainer.segments[segmentID]
		if ok {
			inSegment = true
		}
	}

	builder := strings.Builder{}
	builder.WriteString("- ERROR START -")
	builder.WriteString("\n")
	builder.WriteString("Trace: ")
	builder.WriteString(t.trace)
	builder.WriteString("\n")
	builder.WriteString("Transaction: ")
	builder.WriteString(t.transaction)
	builder.WriteString("\n")
	builder.WriteString("Transaction-Attributes: ")
	builder.WriteString(fmt.Sprintf("%+v", t.attributes))
	builder.WriteString("\n")
	if inSegment {
		builder.WriteString("Segment: ")
		builder.WriteString(t.segmentContainer.segments[segmentID])
		builder.WriteString("\n")
		builder.WriteString("SegmentID: ")
		builder.WriteString(segmentID)
		builder.WriteString("\n")
		builder.WriteString("Segment-Attributes: ")
		builder.WriteString(fmt.Sprintf("%+v", t.segmentContainer.attributes[segmentID]))
		builder.WriteString("\n")
	}
	builder.WriteString("Error: ")
	builder.WriteString(errLog)
	builder.WriteString("\n")
	builder.WriteString("- ERROR END -")

	log.Println(builder.String())

	return nil
}

// Info logs information in the transaction
func (t *LocalTransaction) Info(segmentID string, readCloser io.ReadCloser) error {
	if logLevel == logLevelError {
		return nil
	}
	t.segmentContainer.mutex.Lock()
	defer func() {
		t.segmentContainer.mutex.Unlock()
		closeErr := readCloser.Close()
		if closeErr != nil {
			log.Printf("Telemetry driver local could not close reader while logging Info. Potential resource leak!")
		}
	}()
	t.segmentWriteStart(segmentID)
	infoMsg, err := io.ReadAll(readCloser)
	if err != nil {
		return errors.New("error while reading info message")
	}

	infoLog := string(infoMsg)

	inSegment := false
	if len(segmentID) > 0 {
		_, ok := t.segmentContainer.segments[segmentID]
		if ok {
			inSegment = true
		}
	}

	builder := strings.Builder{}
	builder.WriteString("- INFO START -")
	builder.WriteString("\n")
	builder.WriteString("Trace: ")
	builder.WriteString(t.trace)
	builder.WriteString("\n")
	builder.WriteString("Transaction: ")
	builder.WriteString(t.transaction)
	builder.WriteString("\n")
	builder.WriteString("Transaction-Attributes: ")
	builder.WriteString(fmt.Sprintf("%+v", t.attributes))
	builder.WriteString("\n")
	if inSegment {
		builder.WriteString("Segment: ")
		builder.WriteString(t.segmentContainer.segments[segmentID])
		builder.WriteString("\n")
		builder.WriteString("SegmentID: ")
		builder.WriteString(segmentID)
		builder.WriteString("\n")
		builder.WriteString("Segment-Attributes: ")
		builder.WriteString(fmt.Sprintf("%+v", t.segmentContainer.attributes[segmentID]))
		builder.WriteString("\n")
	}
	builder.WriteString("Message: ")
	builder.WriteString(infoLog)
	builder.WriteString("\n")
	builder.WriteString("- INFO END -")

	fmt.Println(builder.String())

	return nil
}

// Done ends the transaction
func (t *LocalTransaction) Done() error {
	// todo print transaction attributes
	log.Printf("Transaction end: %s \n", t.transaction)

	return nil
}

// CreateTrace creates a trace for the transaction
func (t *LocalTransaction) CreateTrace() (string, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return newUUID.String(), nil
}

// SetTrace sets a trace for the transaction
func (t *LocalTransaction) SetTrace(trace string) error {
	t.trace = trace

	return nil
}

// Trace returns the current trace for the transaction
func (t *LocalTransaction) Trace() (string, error) {
	return t.trace, nil
}

// TraceID returns the current trace for the transaction, this is the same as trace for every instance but apm
func (t *LocalTransaction) TraceID() (string, error) {
	return t.trace, nil
}

// SetTraceID sets a trace for the transaction
func (t *LocalTransaction) SetTraceID(traceID string) error {
	t.trace = traceID
	return nil
}

// CreateProcessID creates a ProcessID for the transaction
func (t *LocalTransaction) CreateProcessID() (string, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return newUUID.String(), nil
}

// SetProcessID sets a ProcessID for the transaction
func (t *LocalTransaction) SetProcessID(processID string) error {
	t.processID = processID

	return nil
}

// ProcessID returns the current ProcessID for the transaction
func (t *LocalTransaction) ProcessID() (string, error) {
	return t.processID, nil
}

// Erase any memory the transaction allocated
func (t *LocalTransaction) Erase() {
	t.attributes = nil
	t.segmentContainer.segments = nil
	t.segmentContainer.attributes = nil

	// we need to collect the garbage manually here because maps in go do have some problems with the garbage collection
	// the runtime.GC method is used to manually free the memory
	// this problem is already known since 2017
	// https://github.com/golang/go/issues/20135
	runtime.GC()
}
