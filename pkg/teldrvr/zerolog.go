package teldrvr

import (
	"errors"
	"fmt"
	"github.com/newrelic/go-agent/v3/integrations/logcontext-v2/zerologWriter"
	"github.com/newrelic/go-agent/v3/newrelic"
	"io"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/plentymarkets/mc-telemetry/pkg/telemetry"
	"github.com/rs/zerolog"
)

/** DRIVER NAME **/
const zerologDriver = "nrZerolog"
const levelInfo = "info"
const levelError = "error"

func init() {
	cfg, err := GetConfig()
	if err != nil {
		log.Fatal(err)
	}

	if !strings.Contains(cfg.GetString("telemetry.driver"), zerologDriver) {
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

	zld := ZeroLogDriver{
		NewRelicApp: nra,
	}

	telemetry.RegisterDriver(newrelicDriver, zld)
	zerolog.SetGlobalLevel(zerolog.DebugLevel)
}

// ZeroLogDriver holds all information the driver needs for telemetry
type ZeroLogDriver struct {
	NewRelicApp *newrelic.Application
}

// Start starts a transaction
func (zld ZeroLogDriver) Start(name string) (telemetry.Transaction, error) {
	writer := zerologWriter.New(os.Stdout, zld.NewRelicApp)
	logger := zerolog.New(writer).With().Timestamp().Logger()
	msg := fmt.Sprintf("Transaction start: %s \n", name)

	zlt := ZeroLogTransaction{
		transaction: logger,
	}

	zlt.logTrace(msg)

	return zlt, nil
}

func (zlt ZeroLogTransaction) logTrace(msg string) {
	preparedLog := zlt.transaction.Info()
	preparedLog.
		Str("traceID", zlt.trace)

	for key, value := range zlt.attributes {
		preparedLog.Any(key, value)
	}

	preparedLog.Msg(msg)
}

// ZeroLogTransaction used for local transactions
type ZeroLogTransaction struct {
	transaction      zerolog.Logger
	segmentContainer ZeroLogSegmentContainer
	attributes       map[string]any
	trace            string
}

// ZeroLogSegmentContainer used for segment handling
type ZeroLogSegmentContainer struct {
	segments   map[string]string         // key = segment ID | value = name of the segment
	attributes map[string]map[string]any // {"segmentID":  {"attributeName": "attribute value"}}
	mutex      *sync.RWMutex
}

// AddTransactionAttribute adds an attribute to the transaction
// - Not thread safe -
func (zlt ZeroLogTransaction) AddTransactionAttribute(key string, value any) error {
	if zlt.attributes == nil {
		zlt.attributes = make(map[string]any)
	}

	val, ok := zlt.attributes[key]
	if ok {
		return fmt.Errorf("transaction attribute '%s' already set with value '%v'", key, val)
	}

	zlt.attributes[key] = value

	return nil
}

// SegmentStart starts a local segment and keeps track of all opened segments
func (zlt ZeroLogTransaction) SegmentStart(segmentID string, name string) error {
	zlt.segmentContainer.mutex.Lock()
	defer zlt.segmentContainer.mutex.Unlock()
	msg := fmt.Sprintf("Segment start: %s \n", name)
	readCloser := io.NopCloser(strings.NewReader(msg))
	err := zlt.Info(segmentID, readCloser)
	if err != nil {
		return err
	}

	if zlt.segmentContainer.segments == nil {
		zlt.segmentContainer.segments = make(map[string]string)
	}

	zlt.segmentContainer.segments[segmentID] = name

	return nil
}

// AddSegmentAttribute adds an attribute to the currently open segment
// - Thread safe -
func (zlt ZeroLogTransaction) AddSegmentAttribute(segmentID string, key string, value any) error {
	zlt.segmentContainer.mutex.Lock()
	defer zlt.segmentContainer.mutex.Unlock()

	segmentName, segmentExist := zlt.segmentContainer.segments[segmentID]
	if !segmentExist {
		return fmt.Errorf("can not add attribute to not existing segment.\nSegmentID: %s\nKey: %s\nValue: %s", segmentID, key, value)
	}

	if zlt.segmentContainer.attributes == nil {
		zlt.segmentContainer.attributes = make(map[string]map[string]any)
	}

	if zlt.segmentContainer.attributes[segmentID] == nil {
		zlt.segmentContainer.attributes[segmentID] = make(map[string]any)
	}

	attribute, attributeExist := zlt.segmentContainer.attributes[segmentID][key]
	if attributeExist {
		return fmt.Errorf("segment attribute already exist.\nSegment: %s\nSegmentID: %s\nKey: %s\nAlready set value: %v", segmentName, segmentID, key, attribute)
	}

	zlt.segmentContainer.attributes[segmentID][key] = value

	return nil
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (zlt ZeroLogTransaction) SegmentEnd(segmentID string) error {
	zlt.segmentContainer.mutex.Lock()
	defer zlt.segmentContainer.mutex.Unlock()
	val, ok := zlt.segmentContainer.segments[segmentID]
	if !ok {
		return fmt.Errorf("Error trying to end segment. Segment is not open.\nSegmentID: %s", segmentID)
	}

	msg := fmt.Sprintf("Segment end: %s", val)
	readCloser := io.NopCloser(strings.NewReader(msg))
	// implement using our info method
	err := zlt.Info(segmentID, readCloser)

	if err != nil {
		return err
	}

	delete(zlt.segmentContainer.segments, segmentID)
	delete(zlt.segmentContainer.attributes, segmentID)

	return nil
}

// Error logs errors in the transaction
func (zlt ZeroLogTransaction) Error(segmentID string, readCloser io.ReadCloser) error {
	return zlt.logMessage(levelInfo, segmentID, readCloser)
}

func (zlt ZeroLogTransaction) logMessage(level string, segmentID string, readCloser io.ReadCloser) error {
	defer func() {
		closeErr := readCloser.Close()
		if closeErr != nil {
			log.Printf("Telemetry driver newRelicZerolog could not close reader while logging Info. Potential resource leak!")
		}
	}()
	// max bytes available for the info message
	msg := make([]byte, telemetry.ErrorBytesSize)

	_, err := readCloser.Read(msg)
	if err != nil {
		return errors.New("error while reading message")
	}

	logMsg := string(msg)
	var preparedLog *zerolog.Event

	switch level {
	case levelInfo:
		preparedLog = zlt.transaction.Info()
		break
	case levelError:
		preparedLog = zlt.transaction.Error()
		break
	default:
		return errors.New("unknown log level")
	}

	preparedLog.
		Str("traceID", zlt.trace).
		Str("segmentID", segmentID).
		Str("action", zlt.segmentContainer.segments[segmentID])

	for key, value := range zlt.segmentContainer.attributes[segmentID] {
		preparedLog.Any(key, value)
	}

	preparedLog.Msg(logMsg)

	return nil
}

// Info logs errors in the transaction
func (zlt ZeroLogTransaction) Info(segmentID string, readCloser io.ReadCloser) error {
	return zlt.logMessage(levelInfo, segmentID, readCloser)
}

// Done ends the transaction
func (zlt ZeroLogTransaction) Done() error {
	msg := fmt.Sprintf("Transaction end: %s \n", zlt.transaction)

	zlt.logTrace(msg)

	return nil
}

// CreateTrace creates a trace for the transaction
func (zlt ZeroLogTransaction) CreateTrace() (string, error) {
	newUUID, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return newUUID.String(), nil
}

// SetTrace sets a trace for the transaction
func (zlt ZeroLogTransaction) SetTrace(trace string) error {
	zlt.trace = trace

	return nil
}

// Trace returns the current ttrace for the transaction
func (zlt ZeroLogTransaction) Trace() (string, error) {
	return zlt.trace, nil
}

// Erase any memory the transaction allocated
func (zlt ZeroLogTransaction) Erase() {
	zlt.attributes = nil
	zlt.segmentContainer.segments = nil
	zlt.segmentContainer.attributes = nil

	// we need to collect the garbage manually here because maps in go do have some problems with the garbage collection
	// the runtime.GC method is used to manually free the memory
	// this problem is already known since 2017
	// https://github.com/golang/go/issues/20135
	runtime.GC()
}
