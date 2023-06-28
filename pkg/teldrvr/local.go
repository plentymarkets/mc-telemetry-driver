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
	ld := LocalDriver{}

	telemetry.RegisterDriver(localDriver, ld)
}

// LocalDriver holds all information the driver needs for telemetry
type LocalDriver struct{}

// Start starts a transaction
func (ld LocalDriver) Start(name string) (telemetry.Transaction, error) {
	log.Printf("Transaction start: %s \n", name)

	lt := LocalTransaction{
		transaction: name,
	}

	return &lt, nil
}

// LocalTransaction used for local transactions
type LocalTransaction struct {
	transaction      string
	segmentContainer LocalSegmentContainer
	attributes       map[string]any
	trace            string
}

// LocalSegmentContainer used for segment handling
type LocalSegmentContainer struct {
	segments   map[string]string
	attributes map[string]map[string]any
	mutex      sync.RWMutex
}

// AddAttribute adds an attribute to the transaction
// - Not thread safe -
func (lt *LocalTransaction) AddTransactionAttribute(key string, value any) error {
	if lt.attributes == nil {
		lt.attributes = make(map[string]any)
	}

	val, ok := lt.attributes[key]
	if ok {
		return fmt.Errorf("transaction attribute '%s' already set with value '%v'", key, val)
	}

	lt.attributes[key] = value

	return nil
}

// SegmentStart starts a local segment and keeps track of all opened segments
func (lt *LocalTransaction) SegmentStart(segmentID string, name string) error {
	lt.segmentContainer.mutex.Lock()
	defer lt.segmentContainer.mutex.Unlock()
	log.Printf("Segment start[%s]: %s \n", segmentID, name)

	if lt.segmentContainer.segments == nil {
		lt.segmentContainer.segments = make(map[string]string)
	}

	lt.segmentContainer.segments[segmentID] = name

	return nil
}

// AddSegmentAttribute adds an attribute to the currently open segment
// - Thread safe -
func (lt *LocalTransaction) AddSegmentAttribute(segmentID string, key string, value any) error {
	lt.segmentContainer.mutex.Lock()
	defer lt.segmentContainer.mutex.Unlock()

	segmentName, segmentExist := lt.segmentContainer.segments[segmentID]
	if !segmentExist {
		return fmt.Errorf("can not add attribute to not existing segment.\nSegmentID: %s\nKey: %s\nValue: %s", segmentID, key, value)
	}

	if lt.segmentContainer.attributes == nil {
		lt.segmentContainer.attributes = make(map[string]map[string]any)
	}

	if lt.segmentContainer.attributes[segmentID] == nil {
		lt.segmentContainer.attributes[segmentID] = make(map[string]any)
	}

	attribute, attributeExist := lt.segmentContainer.attributes[segmentID][key]
	if attributeExist {
		return fmt.Errorf("segment attribute already exist.\nSegment: %s\nSegmentID: %s\nKey: %s\nAlready set value: %v", segmentName, segmentID, key, attribute)
	}

	lt.segmentContainer.attributes[segmentID][key] = value

	return nil
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (lt *LocalTransaction) SegmentEnd(segmentID string) error {
	lt.segmentContainer.mutex.Lock()
	defer lt.segmentContainer.mutex.Unlock()
	val, ok := lt.segmentContainer.segments[segmentID]
	if !ok {
		return fmt.Errorf("Error trying to end segment. Segment is not open.\nSegmentID: %s", segmentID)
	}

	log.Printf("Segment end[%s]: %s\n", segmentID, val)

	delete(lt.segmentContainer.segments, segmentID)
	delete(lt.segmentContainer.attributes, segmentID)

	return nil
}

// Error logs errors in the transaction/segment
func (lt *LocalTransaction) Error(segmentID string, readCloser io.ReadCloser) error {
	// max bytes available for the error message
	errMsg := make([]byte, telemetry.ErrorBytesSize)

	_, err := readCloser.Read(errMsg)
	if err != nil {
		readCloser.Close()
		return errors.New("error while reading err message")
	}
	readCloser.Close()

	errLog := string(errMsg)

	inSegment := false
	if len(segmentID) > 0 {
		_, ok := lt.segmentContainer.segments[segmentID]
		if ok {
			inSegment = true
		}
	}

	builder := strings.Builder{}
	builder.WriteString("- ERROR START -")
	builder.WriteString("\n")
	builder.WriteString("Trace: ")
	builder.WriteString(lt.trace)
	builder.WriteString("\n")
	builder.WriteString("Transaction: ")
	builder.WriteString(lt.transaction)
	builder.WriteString("\n")
	builder.WriteString("Transaction-Attributes: ")
	builder.WriteString(fmt.Sprintf("%+v", lt.attributes))
	builder.WriteString("\n")
	if inSegment {
		builder.WriteString("Segment: ")
		builder.WriteString(lt.segmentContainer.segments[segmentID])
		builder.WriteString("\n")
		builder.WriteString("SegmentID: ")
		builder.WriteString(segmentID)
		builder.WriteString("\n")
		builder.WriteString("Segment-Attributes: ")
		builder.WriteString(fmt.Sprintf("%+v", lt.segmentContainer.attributes[segmentID]))
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
func (lt *LocalTransaction) Info(segmentID string, readCloser io.ReadCloser) error {
	infoMsg, err := io.ReadAll(readCloser)
	if err != nil {
		readCloser.Close()
		return errors.New("error while reading info message")
	}
	readCloser.Close()

	infoLog := string(infoMsg)

	inSegment := false
	if len(segmentID) > 0 {
		_, ok := lt.segmentContainer.segments[segmentID]
		if ok {
			inSegment = true
		}
	}

	builder := strings.Builder{}
	builder.WriteString("- INFO START -")
	builder.WriteString("\n")
	builder.WriteString("Trace: ")
	builder.WriteString(lt.trace)
	builder.WriteString("\n")
	builder.WriteString("Transaction: ")
	builder.WriteString(lt.transaction)
	builder.WriteString("\n")
	builder.WriteString("Transaction-Attributes: ")
	builder.WriteString(fmt.Sprintf("%+v", lt.attributes))
	builder.WriteString("\n")
	if inSegment {
		builder.WriteString("Segment: ")
		builder.WriteString(lt.segmentContainer.segments[segmentID])
		builder.WriteString("\n")
		builder.WriteString("SegmentID: ")
		builder.WriteString(segmentID)
		builder.WriteString("\n")
		builder.WriteString("Segment-Attributes: ")
		builder.WriteString(fmt.Sprintf("%+v", lt.segmentContainer.attributes[segmentID]))
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
func (lt *LocalTransaction) Done() error {
	log.Printf("Transaction end: %s \n", lt.transaction)

	return nil
}

// CreateTrace creates a trace for the transaction
func (lt *LocalTransaction) CreateTrace() (string, error) {
	uuid, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	return uuid.String(), nil
}

// SetTrace sets a trace for the transaction
func (lt *LocalTransaction) SetTrace(trace string) error {
	lt.trace = trace

	return nil
}

// Trace returns the current ttrace for the transaction
func (lt *LocalTransaction) Trace() (string, error) {
	return lt.trace, nil
}

// Erase any memory the transaction allocated
func (lt *LocalTransaction) Erase() {
	lt.attributes = nil
	lt.segmentContainer.segments = nil
	lt.segmentContainer.attributes = nil

	// we need to collect the garbage manually here because maps in go do have some problems with the garbage collection
	// the runtime.GC method is used to manually free the memory
	// this problem is already known since 2017
	// https://github.com/golang/go/issues/20135
	runtime.GC()
}
