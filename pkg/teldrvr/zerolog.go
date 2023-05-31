package teldrvr

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/google/uuid"

	"github.com/plentymarkets/mc-telemetry/pkg/telemetry"
	"github.com/rs/zerolog"
)

/** DRIVER NAME **/
const zerologDriver = "zerolog"

func init() {
	zld := ZeroLogDriver{}
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	telemetry.RegisterDriver(zerologDriver, zld)
}

// ZeroLogDriver holds all information the driver needs for telemetry
type ZeroLogDriver struct{}

// Start starts a transaction
func (zld ZeroLogDriver) Start(name string) (telemetry.Transaction, error) {
	zlt := zerolog.New(os.Stderr).With().Timestamp().Logger()
	msg := fmt.Sprintf("Transaction start: %s \n", name)

	zlt.Info().Msg(msg)

	lt := ZeroLogTransaction{
		transaction: zlt,
	}

	return &lt, nil
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
	segments   []string
	attributes map[string]map[string]any
	mutex      sync.RWMutex
}

// AddTransactionAttribute adds an attribute to the transaction
// - Not thread safe -
func (zlt *ZeroLogTransaction) AddTransactionAttribute(key string, value any) error {
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

// AddSegmentAttribute adds an attribute to the currently open segment
// - Thread safe -
func (zlt *ZeroLogTransaction) AddSegmentAttribute(key string, value any) error {
	zlt.segmentContainer.mutex.Lock()
	defer zlt.segmentContainer.mutex.Unlock()

	if len(zlt.segmentContainer.segments) == 0 {
		return fmt.Errorf("can not add attribute to not existing segment. Key: %s Value: %s", key, value)
	}

	if zlt.segmentContainer.attributes == nil {
		zlt.segmentContainer.attributes = make(map[string]map[string]any)
	}

	currentOpenSegment := zlt.segmentContainer.segments[len(zlt.segmentContainer.segments)-1]

	if zlt.segmentContainer.attributes[currentOpenSegment] == nil {
		zlt.segmentContainer.attributes[currentOpenSegment] = make(map[string]any)
	}

	val, ok := zlt.segmentContainer.attributes[currentOpenSegment][key]
	if ok {
		return fmt.Errorf("segment attribute '%s' already set with value '%v'", key, val)
	}

	zlt.segmentContainer.attributes[currentOpenSegment][key] = value

	return nil
}

// SegmentStart starts a local segment and keeps track of all opened segments
func (zlt *ZeroLogTransaction) SegmentStart(name string) error {

	msg := fmt.Sprintf("Segment start: %s \n", name)

	zlt.transaction.Info().Msg(msg)

	zlt.segmentContainer.segments = append(zlt.segmentContainer.segments, name)

	return nil
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (zlt *ZeroLogTransaction) SegmentEnd() error {
	i := len(zlt.segmentContainer.segments) - 1

	if i < 0 {
		return errors.New("Error trying to end segment. No open segment left")
	}

	msg := fmt.Sprintf("Segment end: %s \n", zlt.segmentContainer.segments[i])

	zlt.transaction.Info().Msg(msg)

	nSegment := make([]string, i)

	copy(nSegment, zlt.segmentContainer.segments[:i])

	zlt.segmentContainer.segments = nSegment

	return nil
}

// Error logs errors in the transaction
func (zlt *ZeroLogTransaction) Error(readCloser io.ReadCloser) error {
	// max bytes available for the error message
	errMsg := make([]byte, telemetry.ErrorBytesSize)

	_, err := readCloser.Read(errMsg)
	if err != nil {
		readCloser.Close()
		return errors.New("error while reading err message")
	}
	readCloser.Close()

	errLog := errors.New(string(errMsg))

	zlt.transaction.Err(errLog)

	return nil
}

// Info logs errors in the transaction
func (zlt *ZeroLogTransaction) Info(readCloser io.ReadCloser) error {
	// max bytes available for the info message
	infoMsg := make([]byte, telemetry.ErrorBytesSize)

	_, err := readCloser.Read(infoMsg)
	if err != nil {
		readCloser.Close()
		return errors.New("error while reading info message")
	}
	readCloser.Close()

	infoLog := string(infoMsg)

	zlt.transaction.Info().Msg(infoLog)

	return nil
}

// Done ends the transaction
func (zlt *ZeroLogTransaction) Done() error {
	msg := fmt.Sprintf("Transaction end: %s \n", zlt.transaction)

	zlt.transaction.Info().Msg(msg)

	return nil
}

// CreateTrace creates a trace for the transaction
func (zlt *ZeroLogTransaction) CreateTrace() (string, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return uuid.String(), nil
}

// SetTrace sets a trace for the transaction
func (zlt *ZeroLogTransaction) SetTrace(trace string) error {
	zlt.trace = trace

	return nil
}

// Trace returns the current ttrace for the transaction
func (zlt *ZeroLogTransaction) Trace() (string, error) {
	return zlt.trace, nil
}

// Erase any memory the transaction allocated
func (zlt *ZeroLogTransaction) Erase() {
	zlt.attributes = nil
	zlt.segmentContainer.segments = nil
	zlt.segmentContainer.attributes = nil

	// we need to collect the garbage manually here because maps in go do have some problems with the garbage collection
	// the runtime.GC method is used to manually free the memory
	// this problem is already known since 2017
	// https://github.com/golang/go/issues/20135
	runtime.GC()
}
