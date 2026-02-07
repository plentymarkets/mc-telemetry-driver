package teldrvr

import (
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestLocalTransaction_InitializeTransaction(t *testing.T) {
	type args struct {
		name string
	}
	initArg := args{
		name: "test transaction",
	}
	tests := []struct {
		name string
		args args
		want LocalTransaction
	}{
		{
			name: "Test LocalTransaction_InitializeTransaction",
			args: initArg,
			want: LocalTransaction{
				transaction: initArg.name,
				attributes:  make(map[string]any),
				trace:       "",
				processID:   "",
				segmentContainer: LocalSegmentContainer{
					segments:               make(map[string]string),
					attributes:             make(map[string]map[string]any),
					mutex:                  sync.RWMutex{},
					segmentsStartWasLogged: make(map[string]struct{}),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			got, err := driver.InitializeTransaction(tt.args.name)
			if err != nil {
				t.Errorf("LocalTransaction_InitializeTransaction() error = %v", err)
				return
			}

			if !reflect.DeepEqual(got, &tt.want) {
				t.Errorf("LocalTransaction_InitializeTransaction() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_AddTransactionAttribute(t *testing.T) {
	type args struct {
		key   string
		value any
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Test LocalTransaction_AddTransactionAttribute",
			args: args{
				key:   "test attribute",
				value: "test value",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			if got := transaction.AddTransactionAttribute(tt.args.key, tt.args.value); got != tt.want {
				t.Errorf("LocalTransaction_AddTransactionAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_SegmentStart(t *testing.T) {
	type args struct {
		segmentID string
		name      string
	}

	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Test LocalTransaction_SegmentStart",
			args: args{
				segmentID: uuid.NewString(),
				name:      "test segment",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			if got := transaction.SegmentStart(tt.args.segmentID, tt.args.name); got != tt.want {
				t.Errorf("LocalTransaction_SegmentStart() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_AddSegmentAttribute(t *testing.T) {
	type args struct {
		segmentID string
		key       string
		value     any
	}

	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Test LocalTransaction_AddSegmentAttribute",
			args: args{
				segmentID: uuid.NewString(),
				key:       "test attribute",
				value:     "test value",
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			transaction.SegmentStart(tt.args.segmentID, "test segment")
			if got := transaction.AddSegmentAttribute(tt.args.segmentID, tt.args.key, tt.args.value); got != tt.want {
				t.Errorf("LocalTransaction_AddSegmentAttribute() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_SegmentEnd(t *testing.T) {
	type args struct {
		segmentID string
	}

	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Test LocalTransaction_SegmentEnd",
			args: args{
				segmentID: uuid.NewString(),
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			transaction.SegmentStart(tt.args.segmentID, "test segment")
			if got := transaction.SegmentEnd(tt.args.segmentID); got != tt.want {
				t.Errorf("LocalTransaction_SegmentEnd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_Done(t *testing.T) {
	tests := []struct {
		name string
		want error
	}{
		{
			name: "Test LocalTransaction_Done",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			if got := transaction.Done(); got != tt.want {
				t.Errorf("LocalTransaction_Done() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_Info(t *testing.T) {
	type args struct {
		segmentID  string
		readCloser io.ReadCloser
	}

	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Test LocalTransaction_Info",
			args: args{
				segmentID:  uuid.NewString(),
				readCloser: io.NopCloser(strings.NewReader("test message")),
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			transaction.SegmentStart(tt.args.segmentID, "test segment")
			if got := transaction.Info(tt.args.segmentID, tt.args.readCloser); got != tt.want {
				t.Errorf("LocalTransaction_Info() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_Error(t *testing.T) {
	type args struct {
		segmentID  string
		readCloser io.ReadCloser
	}

	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Test LocalTransaction_Error",
			args: args{
				segmentID:  uuid.NewString(),
				readCloser: io.NopCloser(strings.NewReader("test message")),
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			transaction.SegmentStart(tt.args.segmentID, "test segment")
			if got := transaction.Error(tt.args.segmentID, tt.args.readCloser); got != tt.want {
				t.Errorf("LocalTransaction_Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_SetTraceID(t *testing.T) {
	type args struct {
		traceID string
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Test LocalTransaction_SetTraceID",
			args: args{
				traceID: uuid.NewString(),
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			if got := transaction.SetTraceID(tt.args.traceID); got != tt.want {
				t.Errorf("LocalTransaction_SetTraceID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_Trace(t *testing.T) {
	trace := uuid.NewString()
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Test LocalTransaction_Trace",
			want: trace,
		},
	}

	driver := LocalDriver{}
	transaction, _ := driver.InitializeTransaction("test transaction")
	transaction.SetTraceID(trace)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := transaction.Trace(); got != tt.want {
				t.Errorf("LocalTransaction_Trace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_TraceID(t *testing.T) {
	trace := uuid.NewString()
	tests := []struct {
		name string
		want string
	}{
		{
			name: "Test LocalTransaction_TraceID",
			want: trace,
		},
	}

	driver := LocalDriver{}
	transaction, _ := driver.InitializeTransaction("test transaction")
	transaction.SetTraceID(trace)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, _ := transaction.TraceID(); got != tt.want {
				t.Errorf("LocalTransaction_TraceID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_CreateProcessID(t *testing.T) {
	tests := []struct {
		name string
		want error
	}{
		{
			name: "Test LocalTransaction_CreateProcessID",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			if _, got := transaction.CreateProcessID(); got != tt.want {
				t.Errorf("LocalTransaction_CreateProcessID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_SetProcessID(t *testing.T) {
	type args struct {
		processID string
	}
	tests := []struct {
		name string
		args args
		want error
	}{
		{
			name: "Test LocalTransaction_SetProcessID",
			args: args{
				processID: uuid.NewString(),
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver := LocalDriver{}
			transaction, _ := driver.InitializeTransaction("test transaction")
			if got := transaction.SetProcessID(tt.args.processID); got != tt.want {
				t.Errorf("LocalTransaction_SetProcessID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLocalTransaction_ProcessID(t *testing.T) {
	processID := uuid.NewString()
	tests := []struct {
		name      string
		want      string
		wantError error
	}{
		{
			name:      "Test LocalTransaction_ProcessID",
			want:      processID,
			wantError: nil,
		},
	}

	driver := LocalDriver{}
	transaction, _ := driver.InitializeTransaction("test transaction")
	transaction.SetProcessID(processID)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := transaction.ProcessID()
			if got != tt.want {
				t.Errorf("LocalTransaction_ProcessID() = %v, want %v", got, tt.want)
			}
			if err != tt.wantError {
				t.Errorf("LocalTransaction_ProcessID() error = %v", err)
			}
		})
	}
}
