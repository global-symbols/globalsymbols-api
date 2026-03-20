package analytics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

const maxCapturedBodyBytes = 4096

type responseRecorder struct {
	writer      http.ResponseWriter
	statusCode  int
	wroteHeader bool
	body        []byte
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		writer:     w,
		statusCode: http.StatusOK,
	}
}

func (r *responseRecorder) Header() http.Header {
	return r.writer.Header()
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}

	r.statusCode = status
	r.wroteHeader = true
	r.writer.WriteHeader(status)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}

	r.capture(p)
	return r.writer.Write(p)
}

func (r *responseRecorder) Flush() {
	flusher, ok := r.writer.(http.Flusher)
	if !ok {
		return
	}

	if !r.wroteHeader {
		r.WriteHeader(http.StatusOK)
	}
	flusher.Flush()
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.writer.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("http.Hijacker not supported")
	}
	return hijacker.Hijack()
}

func (r *responseRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := r.writer.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (r *responseRecorder) StatusCode() int {
	return r.statusCode
}

func (r *responseRecorder) Body() []byte {
	out := make([]byte, len(r.body))
	copy(out, r.body)
	return out
}

func (r *responseRecorder) capture(p []byte) {
	if len(r.body) >= maxCapturedBodyBytes {
		return
	}

	remaining := maxCapturedBodyBytes - len(r.body)
	if len(p) > remaining {
		p = p[:remaining]
	}
	r.body = append(r.body, p...)
}
