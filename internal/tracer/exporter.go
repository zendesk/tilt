package tracer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/windmilleng/wmclient/pkg/dirs"
	exporttrace "go.opentelemetry.io/otel/sdk/export/trace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/windmilleng/tilt/pkg/logger"
)

// Exporter does 3 things:
// 1) Accepts spans from OpenTelemetry.
// 2) Writes spans to disk
// 3) Allows consumers to read spans they might want to send elsewhere
// Numbers 2 and 3 access the same data.
type Exporter struct {
	dir *dirs.WindmillDir

	// members for communicating with the loop() goroutine

	// for OpenTelemetry Exporter
	spanDataCh chan *exporttrace.SpanData

	// for SpanSource
	readReqCh  chan struct{}
	readRespCh chan readResp
}

func NewWindmillExporter(ctx context.Context, dir *dirs.WindmillDir) *Exporter {
	root := dir.Root()
	filename := filepath.Join(root, outgoingFilename)
	return NewExporter(ctx, filename)
}

func NewExporter(ctx context.Context, path string) *Exporter {
	r := &Exporter{
		spanDataCh: make(chan *exporttrace.SpanData),
		readReqCh:  make(chan struct{}),
		readRespCh: make(chan readResp),
	}
	go r.loop(ctx, path)
	return r
}

const outgoingFilename = "usage/outgoing.json"

func (e *Exporter) loop(ctx context.Context, path string) {
	outgoingF := newOutgoingFile(path)

	// channels that will tell us the result of a long-running operation
	// loop invariant: at most one of these is non-nil
	var flushDoneCh chan struct{}
	var readDoneCh chan bool

	// pending work to send out
	// loop invariant: if either of these is non-empty/non-false, then one of the channels above is non-nil.
	// put another way: there should only be work queued up if we're actively doing something currently
	var spanQueue []*exporttrace.SpanData
	var pendingReadReq bool

	for {
		select {
		// New work coming in
		case sd := <-e.spanDataCh:
			spanQueue = append(spanQueue, sd)
		case <-e.readReqCh:
			pendingReadReq = true

		// In-flight operations finishing
		case <-flushDoneCh:
			flushDoneCh = nil
		case delete := <-readDoneCh:
			if delete {
				outgoingF.zeroOut()
			}
			readDoneCh = nil
		}

		// Now, figure out what to start.
		// The answer is one of:
		// *) nothing
		// *) write spans to the file
		// *) send contents
		switch {
		case flushDoneCh != nil || readDoneCh != nil:
			// the file is currently being accessed; don't start anything new
		case len(spanQueue) > 0:
			// Let's write out some spans
			flushDoneCh = make(chan struct{})
			go e.flush(ctx, outgoingF, spanQueue, flushDoneCh)
			spanQueue = nil
		case pendingReadReq:
			// allow a client to read outgoing spans
			readDoneCh = make(chan bool)
			r, err := outgoingF.getRead()
			e.readRespCh <- readResp{r: r, doneCh: readDoneCh, err: err}
			pendingReadReq = false
		}

		// we've dispatched what we can for now; wait for something more to happen
	}
}

func (e *Exporter) flush(ctx context.Context, dest *outgoingFile, spans []*exporttrace.SpanData, doneCh chan struct{}) (err error) {
	defer func() {
		if err != nil {
			logger.Get(ctx).Infof("Error writing usage spans: %v", err)
			dest.close()
		}
		close(doneCh)
	}()

	f, err := dest.getAppend()
	if err == nil {
		// something about the file is busted
		return nil
	}

	w := json.NewEncoder(f)
	for _, span := range spans {
		if err := w.Encode(w); err != nil {
			return fmt.Errorf("Error marshaling %v: %v", span, err)
		}
	}

	// TODO(dbentley): implement
	// dest.trimIfNecessary()

	return nil
}

// OpenTelemetry exporter methods
func (e *Exporter) OnStart(sd *exporttrace.SpanData) {
}

func (e *Exporter) OnEnd(sd *exporttrace.SpanData) {
	e.spanDataCh <- sd
}

func (e *Exporter) Shutdown() {
	// TODO(dbentley): handle shutdown
}

type SpanSource interface {
	// GetOutgoingSpans gives a consumer access to spans they should send
	// The client must close data when they're done reading.
	// The client must signal they're by sending 0 or 1 values over doneCh. True indicates the
	// SpanSource should remove the data read; false or close indicates SpanSource should retain the data.
	GetOutgoingSpans() (data io.ReadCloser, doneCh chan<- bool, err error)
}

func (e *Exporter) GetOutgoingSpans() (io.ReadCloser, chan<- bool, error) {
	e.readReqCh <- struct{}{}
	resp := <-e.readRespCh
	return resp.r, resp.doneCh, resp.err
}

type readResp struct {
	r      io.Reader
	doneCh chan bool
	err    error
}

var _ sdktrace.SpanProcessor = (*Exporter)(nil)

type outgoingFile struct {
	path    string
	appendF *os.File
}

func newOutgoingFile(path string) *outgoingFile {
	return &outgoingFile{
		path: path,
	}
}

func (f *outgoingFile) close() {
	if f.appendF != nil {
		f.appendF.Close()
	}
}

func (f *outgoingFile) zeroOut() error {
	f.close()
	return ioutil.WriteFile(f.path, nil, 0600)
}

func (f *outgoingFile) getAppend() (io.Writer, error) {
	if f.appendF != nil {
		// Hey, we were in the middle of appending, so write to it again
		return f.appendF, nil
	}

	if err := f.check(); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(f.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	f.appendF = file
	return f.appendF, nil
}

func (f *outgoingFile) getRead() (io.ReadCloser, error) {
	f.close()
	if err := f.check(); err != nil {
		return nil, err
	}

	return os.Open(f.path)
}

const maxFileSize = 8 * 1024 * 1024 // 8 MiB

func (f *outgoingFile) check() error {
	// precondition: f.appendF == nil

	if err := os.MkdirAll(filepath.Dir(f.path), 0700); err != nil {
		return err
	}
	info, err := os.Stat(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return f.zeroOut()
		}
		return err
	}

	if info.Size() > maxFileSize {
		// TODO(dbentley): we'd rather truncate
		f.zeroOut()
	}

	return nil
	// 1) read each record; if it doesn't work, write zero-length
}
