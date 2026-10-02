package jlcpcb

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

// The file tests copy the live headers of downloadByFileSystemAccessId
// (2026-10-02). The server sends "application/x-msdownload" for images, and
// it answers an unknown id with HTTP 200 and a gzip JSON body.

// unknownFileResponse is the live answer for an unknown file access id.
const unknownFileResponse = `{"code":500,"data":null,"message":"System error. Please try again later."}`

// testJPEG returns n bytes that start with a JPEG header.
func testJPEG(n int) []byte {
	data := make([]byte, n)
	copy(data, []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00\x01\x02\x00\x00\x01"))
	for i := 16; i < n; i++ {
		data[i] = byte(i % 251)
	}
	return data
}

// fileServer is a test server for file downloads. It counts the requests
// and calls handle for each request.
type fileServer struct {
	requests atomic.Int32
	server   *httptest.Server
	lastPath atomic.Value
}

func newFileServer(t *testing.T, handle func(w http.ResponseWriter, r *http.Request, n int)) *fileServer {
	t.Helper()
	fs := &fileServer{}
	fs.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(fs.requests.Add(1))
		fs.lastPath.Store(r.Method + " " + r.URL.Path)
		handle(w, r, n)
	}))
	t.Cleanup(fs.server.Close)
	return fs
}

// client returns a client that sends every request to the test server, with
// up to 3 retries.
func (fs *fileServer) client(opts ...ClientOption) *Client {
	base := []ClientOption{
		WithAPIRoot(fs.server.URL),
		WithHTTPClient(fs.server.Client()),
		WithRateLimit(1000),
		WithRetryConfig(RetryConfig{MaxRetries: 3, InitialBackoff: 1, MaxBackoff: 1, BackoffMultiplier: 1}),
		WithoutCache(),
	}
	return NewClient(append(base, opts...)...)
}

// writeGzip writes body with gzip content encoding.
func writeGzip(t *testing.T, w http.ResponseWriter, contentType string, body []byte) {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(body); err != nil {
		t.Errorf("gzip body: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Errorf("gzip close: %v", err)
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Encoding", "gzip")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	_, _ = w.Write(buf.Bytes())
}

func readAllAndClose(t *testing.T, body io.ReadCloser) []byte {
	t.Helper()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if err := body.Close(); err != nil {
		t.Errorf("close body: %v", err)
	}
	return data
}

func TestFileOpenSniffsImage(t *testing.T) {
	image := testJPEG(3225)
	fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("Content-Type", "application/x-msdownload;charset=UTF-8")
		w.Header().Set("Content-Disposition", "attachment; filename=C1525-%E6%AD%A3%E9%9D%A2.jpg")
		w.Header().Set("Content-Length", strconv.Itoa(len(image)))
		_, _ = w.Write(image)
	})

	body, info, err := fs.client().File.Open(context.Background(), " 8552476005450338304 ")
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if got := readAllAndClose(t, body); !bytes.Equal(got, image) {
		t.Errorf("body has %d bytes, want the %d image bytes", len(got), len(image))
	}
	want := FileInfo{ContentType: "image/jpeg", FileName: "C1525-正面.jpg", Size: int64(len(image))}
	if *info != want {
		t.Errorf("info = %+v, want %+v", *info, want)
	}
	if got, want := fs.lastPath.Load(), "GET "+fileDownloadPath+"8552476005450338304"; got != want {
		t.Errorf("request = %v, want %s", got, want)
	}
}

func TestFileOpenChunkedPDF(t *testing.T) {
	pdf := append([]byte("%PDF-1.3\n%\xc4\xe5\xf2\xe5\xeb\xa7\n"), bytes.Repeat([]byte("0 0 obj\n"), 200)...)
	fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("Content-Type", "application/pdf;charset=UTF-8")
		w.Header().Set("Content-Disposition", "inline; filename=C1525.pdf")
		// A flush before the body makes the response chunked.
		w.(http.Flusher).Flush()
		_, _ = w.Write(pdf[:100])
		w.(http.Flusher).Flush()
		_, _ = w.Write(pdf[100:])
	})

	body, info, err := fs.client().File.Open(context.Background(), "8579707269996871680")
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if got := readAllAndClose(t, body); !bytes.Equal(got, pdf) {
		t.Errorf("body has %d bytes, want the %d PDF bytes", len(got), len(pdf))
	}
	want := FileInfo{ContentType: "application/pdf", FileName: "C1525.pdf", Size: -1}
	if *info != want {
		t.Errorf("info = %+v, want %+v", *info, want)
	}
}

// TestFileOpenFileNameForms checks the file name forms of the live server
// and the RFC 5987 form.
func TestFileOpenFileNameForms(t *testing.T) {
	tests := []struct {
		name        string
		disposition string
		want        string
	}{
		{"percent-encoded UTF-8", "attachment; filename=C1525-%E6%AD%A3%E9%9D%A2.jpg", "C1525-正面.jpg"},
		{"plain ASCII", "attachment; filename=C25744-front.jpg", "C25744-front.jpg"},
		{"RFC 5987", "attachment; filename*=UTF-8''C1525-%E6%AD%A3%E9%9D%A2.jpg", "C1525-正面.jpg"},
		{"raw GBK with a folder prefix", "attachment; filename=C2040/C2040-\xd5\xfd\xc3\xe6.jpg", "C2040-_.jpg"},
		{"no file name", "attachment", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			image := testJPEG(100)
			fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
				w.Header().Set("Content-Type", "application/x-msdownload;charset=UTF-8")
				w.Header().Set("Content-Disposition", tt.disposition)
				_, _ = w.Write(image)
			})
			body, info, err := fs.client().File.Open(context.Background(), "8583419803382398976")
			if err != nil {
				t.Fatalf("open failed: %v", err)
			}
			readAllAndClose(t, body)
			if info.FileName != tt.want || info.ContentType != "image/jpeg" {
				t.Errorf("info = %+v, want file name %q and image/jpeg", *info, tt.want)
			}
		})
	}
}

// TestFileOpenUnknownID checks the live answer for an unknown id: HTTP 200
// with a gzip JSON body and envelope code 500. The test runs with the
// transport that decompresses the body and with a transport that does not.
func TestFileOpenUnknownID(t *testing.T) {
	for _, disableCompression := range []bool{false, true} {
		name := "transport gzip"
		if disableCompression {
			name = "manual gzip"
		}
		t.Run(name, func(t *testing.T) {
			fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
				writeGzip(t, w, "application/json", []byte(unknownFileResponse))
			})
			httpClient := fs.server.Client()
			httpClient.Transport = &http.Transport{DisableCompression: disableCompression}

			body, info, err := fs.client(WithHTTPClient(httpClient)).File.Open(context.Background(), "8552476004846223360")
			if !errors.Is(err, ErrNotFound) || !errors.Is(err, ErrServer) {
				t.Fatalf("error = %v, want ErrNotFound and ErrServer", err)
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.Code != 500 || apiErr.StatusCode != http.StatusOK {
				t.Errorf("error = %v, want an APIError with code 500 and status 200", err)
			}
			if body != nil || info != nil {
				t.Errorf("body %v, info %v, want nil", body, info)
			}
			// The answer for an unknown id does not change, so Open does
			// not retry.
			if n := fs.requests.Load(); n != 1 {
				t.Errorf("server got %d requests, want 1", n)
			}
		})
	}
}

func TestFileOpenEnvelopeErrors(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		want  error
		notIs error
	}{
		{"rejected", `{"code":101,"data":null,"message":"unknown system error"}`, ErrRejected, ErrNotFound},
		{"rate limited", ` {"code":429,"message":"too many requests"}`, ErrRateLimited, ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			})
			_, _, err := fs.client().File.Open(context.Background(), "1")
			if !errors.Is(err, tt.want) || errors.Is(err, tt.notIs) {
				t.Fatalf("error = %v, want %v and not %v", err, tt.want, tt.notIs)
			}
			if n := fs.requests.Load(); n != 1 {
				t.Errorf("server got %d requests, want 1", n)
			}
		})
	}
}

// TestFileOpenJSONFile checks that a JSON body without an envelope code is
// a file, and that Open returns all of its bytes.
func TestFileOpenJSONFile(t *testing.T) {
	file := []byte(`{"name":"` + string(bytes.Repeat([]byte("x"), 2000)) + `","code":"text"}`)
	fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		_, _ = w.Write(file)
	})
	body, info, err := fs.client().File.Open(context.Background(), "1")
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if got := readAllAndClose(t, body); !bytes.Equal(got, file) {
		t.Errorf("body has %d bytes, want %d", len(got), len(file))
	}
	if info.ContentType != "text/plain; charset=utf-8" {
		t.Errorf("content type = %q", info.ContentType)
	}
}

func TestFileOpenEmptyBody(t *testing.T) {
	fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		w.Header().Set("Content-Type", "application/pdf")
	})
	if _, _, err := fs.client().File.Open(context.Background(), "1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestFileOpenRetriesHTTPErrors(t *testing.T) {
	image := testJPEG(600)
	fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, n int) {
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(image)
	})
	body, info, err := fs.client().File.Open(context.Background(), "1")
	if err != nil {
		t.Fatalf("open failed: %v", err)
	}
	if got := readAllAndClose(t, body); !bytes.Equal(got, image) || info.ContentType != "image/jpeg" {
		t.Errorf("body has %d bytes, content type %q", len(got), info.ContentType)
	}
	if n := fs.requests.Load(); n != 2 {
		t.Errorf("server got %d requests, want 2", n)
	}
}

func TestFileOpenHTTPNotFound(t *testing.T) {
	fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		http.NotFound(w, r)
	})
	_, _, err := fs.client().File.Open(context.Background(), "1")
	var apiErr *APIError
	if !errors.Is(err, ErrNotFound) || !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("error = %v, want ErrNotFound with status 404", err)
	}
	if n := fs.requests.Load(); n != 1 {
		t.Errorf("server got %d requests, want 1", n)
	}
}

func TestFileOpenInvalidID(t *testing.T) {
	fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		_, _ = w.Write(testJPEG(10))
	})
	client := fs.client()
	for _, id := range []string{"", " ", "abc", "12-3", "../1", "1/2"} {
		if _, _, err := client.File.Open(context.Background(), id); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("Open(%q) error = %v, want ErrInvalidRequest", id, err)
		}
	}
	if n := fs.requests.Load(); n != 0 {
		t.Errorf("server got %d requests, want 0", n)
	}
}

func TestFileOpenCanceledContext(t *testing.T) {
	fs := newFileServer(t, func(w http.ResponseWriter, r *http.Request, _ int) {
		_, _ = w.Write(testJPEG(10))
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := fs.client(WithRateLimit(0.001)).File.Open(ctx, "1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if n := fs.requests.Load(); n != 0 {
		t.Errorf("server got %d requests, want 0", n)
	}
}

func TestDispositionFileName(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"attachment; filename=C1525-%E6%AD%A3%E9%9D%A2.jpg", "C1525-正面.jpg"},
		{"inline; filename=C1525.pdf", "C1525.pdf"},
		{"attachment; filename=C2040/C2040-\xd5\xfd\xc3\xe6.jpg", "C2040-_.jpg"},
		{"attachment; filename*=UTF-8''C1525-%E6%AD%A3%E9%9D%A2.jpg", "C1525-正面.jpg"},
		{`attachment; filename="fallback.jpg"; filename*=utf-8'en'%E6%AD%A3.jpg`, "正.jpg"},
		{`attachment; filename*=UTF-8''%E6%AD%A3.jpg; filename="fallback.jpg"`, "正.jpg"},
		{"attachment; filename*=ISO-8859-1''caf%E9.pdf", "café.pdf"},
		{"attachment; filename*=UTF-8''%FF.pdf; filename=plain.pdf", "plain.pdf"},
		{"attachment; filename*=GBK''%D5%FD.jpg; filename=plain.jpg", "plain.jpg"},
		{`attachment; filename="a\"b.pdf"`, `a"b.pdf`},
		{`attachment; filename="x;y.pdf"`, "x;y.pdf"},
		{`attachment; FILENAME = "spaced name.pdf" `, "spaced name.pdf"},
		{`attachment; filename=dir\sub\C1.pdf`, "C1.pdf"},
		{"attachment; filename=100%.pdf", "100%.pdf"},
		{"attachment; filename=%2E%2E", ""},
		{"attachment; filename=..", ""},
		{"attachment; filename=a%0Ab.pdf", "ab.pdf"},
		{"attachment; filename=", ""},
		{"attachment", ""},
		{"", ""},
	}
	for _, tt := range tests {
		if got := dispositionFileName(tt.header); got != tt.want {
			t.Errorf("dispositionFileName(%q) = %q, want %q", tt.header, got, tt.want)
		}
	}
}
