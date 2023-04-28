package teldrvr

import (
	"io"
	"log"

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
func (ld LocalDriver) Start(name string) telemetry.Transaction {
	log.Printf("Transaction start: %s \n", name)

	lt := LocalTransaction{
		transaction: name,
	}

	return &lt
}

// LocalTransaction used for local transactions
type LocalTransaction struct {
	transaction string
	segments    []string
	attributes  map[string]any
}

// AddAttribute adds an attribute to the transaction
func (lt *LocalTransaction) AddAttribute(key string, value any) {
	val, ok := lt.attributes[key]
	if ok {
		log.Printf("Attribute '%s' already set with value '%v'", key, val)
		return
	}

	lt.attributes[key] = value
}

// SegmentStart starts a local segment and keeps track of all opened segments
func (lt *LocalTransaction) SegmentStart(name string) {
	log.Printf("Segment start: %s \n", name)

	lt.segments = append(lt.segments, name)
}

// SegmentEnd ends the current open segment (LIFO) and keeps track of all opened segments
func (lt *LocalTransaction) SegmentEnd() {
	i := len(lt.segments) - 1

	log.Printf("Segment end: %s \n", lt.segments[i])

	nSegment := make([]string, i)

	copy(nSegment, lt.segments[:i])

	lt.segments = nSegment
}

// Error logs errors in the transaction
func (lt *LocalTransaction) Error(readCloser io.ReadCloser) {
	// max bytes available for the error message
	errMsg := make([]byte, telemetry.ErrorBytesSize)

	_, err := readCloser.Read(errMsg)
	if err != nil {
		readCloser.Close()
		log.Panicln("error while reading err message")
	}
	readCloser.Close()

	errLog := string(errMsg)

	log.Printf("- ERROR START -\nTransaction: %s\nSegment: %s\nMessage: %s\nAttributes: %+v\n- ERROR END -\n",
		lt.transaction,
		lt.segments[len(lt.segments)-1],
		errLog,
		lt.attributes)
}

// Info logs information in the transaction
func (lt *LocalTransaction) Info(readCloser io.ReadCloser) {
	infoMsg, err := io.ReadAll(readCloser)
	if err != nil {
		readCloser.Close()
		log.Panicln("error while reading info message")
	}
	readCloser.Close()

	infoLog := string(infoMsg)

	log.Printf("- INFO START -\nTransaction: %s\nSegment: %s\nMessage: %s\nAttributes: %+v\n- INFO END -\n",
		lt.transaction,
		lt.segments[len(lt.segments)-1],
		infoLog,
		lt.attributes)
}

// Done ends the transaction
func (lt *LocalTransaction) Done() {
	log.Printf("Transaction end: %s \n", lt.transaction)
}
