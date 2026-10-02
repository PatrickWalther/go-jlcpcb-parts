package jlcpcb

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"
)

const (
	// sniffLength is the number of bytes that http.DetectContentType reads.
	sniffLength = 512
	// maxFileErrorBody is the largest body that Open reads to find an
	// envelope error. The live error body is 74 bytes.
	maxFileErrorBody = 64 << 10
)

// FileService downloads the files that JLCPCB hosts, such as part images
// and datasheet copies, by file access id.
type FileService service

// FileInfo describes a file that FileService.Open returns.
type FileInfo struct {
	// ContentType is the media type that http.DetectContentType finds in
	// the first 512 bytes, for example "image/jpeg" or "application/pdf".
	// Open does not use the Content-Type of the server, because the server
	// sends "application/x-msdownload" for images.
	ContentType string
	// FileName is the file name from the Content-Disposition header,
	// without a folder prefix, for example "C1525-正面.jpg". It is "" when
	// the server sends no file name. Bytes that are not valid UTF-8, for
	// example a GBK name, become "_".
	FileName string
	// Size is the Content-Length of the file in bytes. It is -1 when the
	// server sends no length, for example for a chunked or compressed
	// response.
	Size int64
}

// Open downloads the file with the file access id accessID, for example
// "8552476004850417664". It sends one GET request to the URL of FileURL
// below the API root. The caller must close the returned body.
//
// Open waits for the rate limiter before the request. It retries an HTTP
// 429 or 5xx answer and a network timeout like the other requests.
//
// The Timeout of the HTTP client (30 seconds by default) limits the full
// download. The limit includes the time to read the body after Open
// returns. A large datasheet (for example 8 MB) can need more time on a
// slow connection. Then a read from the body fails. To download large
// files, set a longer Timeout or no Timeout with WithHTTPClient, and limit
// the download with ctx. A cancel of ctx also stops a body read.
//
// Open reads the first 512 bytes to find the content type. The returned
// body still starts at the first byte. Open does not check the content
// type. Check FileInfo.ContentType before you use the file.
//
// Open returns ErrInvalidRequest when accessID is not all digits, and it
// sends no request. The server answers an unknown id with HTTP 200 and the
// JSON body {"code":500,...}. Open then returns an error that matches
// ErrNotFound and ErrServer, because the answer does not show which one
// applies. Open does not retry this answer. An empty body also gives
// ErrNotFound.
func (s *FileService) Open(ctx context.Context, accessID string) (io.ReadCloser, *FileInfo, error) {
	id := strings.TrimSpace(accessID)
	if !isAccessID(id) {
		return nil, nil, fmt.Errorf("%w: %q is not a file access id (digits only)", ErrInvalidRequest, accessID)
	}

	c := s.client
	endpoint := c.apiRoot + fileDownloadPath + id
	var body io.ReadCloser
	var info *FileInfo
	err := c.withRetry(ctx, retryUnlessEnvelopeError, func() (int, error) {
		var statusCode int
		var err error
		body, info, statusCode, err = c.openFile(ctx, endpoint, id)
		return statusCode, err
	})
	if err != nil {
		return nil, nil, err
	}
	return body, info, nil
}

// openFile sends one file request. It returns the body, the file info and
// the HTTP status code. On an error, it closes the response body.
func (c *Client) openFile(ctx context.Context, endpoint, accessID string) (io.ReadCloser, *FileInfo, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("jlcpcb: failed to create request: %w", err)
	}
	// Do not set Accept-Encoding. The transport then asks for gzip itself
	// and decompresses the body.
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", "https://jlcpcb.com/parts")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("jlcpcb: request failed: %w", err)
	}
	statusCode := resp.StatusCode
	if statusCode < 200 || statusCode >= 300 {
		details, _ := io.ReadAll(io.LimitReader(resp.Body, maxFileErrorBody))
		_ = resp.Body.Close()
		return nil, nil, statusCode, &APIError{
			StatusCode: statusCode,
			Code:       statusCode,
			Message:    http.StatusText(statusCode),
			Details:    string(details),
		}
	}

	body, size, err := decodedFileBody(resp)
	if err != nil {
		_ = resp.Body.Close()
		return nil, nil, statusCode, err
	}

	head := make([]byte, sniffLength)
	n, err := io.ReadFull(body, head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		_ = body.Close()
		return nil, nil, statusCode, fmt.Errorf("jlcpcb: failed to read file %s: %w", accessID, err)
	}
	head = head[:n]
	if len(head) == 0 {
		_ = body.Close()
		return nil, nil, statusCode, fmt.Errorf("%w: file %s is empty", ErrNotFound, accessID)
	}

	// The server sends an error as a JSON envelope with HTTP 200. Read a
	// JSON body in full to find the envelope code.
	if trimmed := bytes.TrimLeft(head, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '{' {
		rest, err := io.ReadAll(io.LimitReader(body, maxFileErrorBody-int64(len(head))))
		if err != nil {
			_ = body.Close()
			return nil, nil, statusCode, fmt.Errorf("jlcpcb: failed to read file %s: %w", accessID, err)
		}
		head = append(head, rest...)
		if apiErr := fileEnvelopeError(head, statusCode); apiErr != nil {
			_ = body.Close()
			if apiErr.Code == http.StatusInternalServerError {
				return nil, nil, statusCode, fmt.Errorf("%w: file %s: %w", ErrNotFound, accessID, apiErr)
			}
			return nil, nil, statusCode, apiErr
		}
	}

	info := &FileInfo{
		ContentType: http.DetectContentType(head),
		FileName:    dispositionFileName(resp.Header.Get("Content-Disposition")),
		Size:        size,
	}
	return &fileReader{Reader: io.MultiReader(bytes.NewReader(head), body), closer: body}, info, statusCode, nil
}

// decodedFileBody returns the body of resp without the gzip content
// encoding, and the size of the body (-1 when it is not known). The
// transport decompresses the body itself when it asked for gzip. A custom
// transport can leave the body compressed.
func decodedFileBody(resp *http.Response) (io.ReadCloser, int64, error) {
	if resp.Uncompressed || !strings.EqualFold(strings.TrimSpace(resp.Header.Get("Content-Encoding")), "gzip") {
		return resp.Body, resp.ContentLength, nil
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, 0, fmt.Errorf("jlcpcb: failed to create gzip reader: %w", err)
	}
	return &gzipBody{Reader: gz, body: resp.Body}, -1, nil
}

// fileEnvelopeError returns the API error of a JSON envelope body with a
// code other than 200. It returns nil when body is not a JSON object with a
// numeric code, or when the code is 200.
func fileEnvelopeError(body []byte, statusCode int) *APIError {
	var envelope struct {
		Code    *int            `json:"code"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.Code == nil || *envelope.Code == 200 {
		return nil
	}
	return &APIError{
		StatusCode: statusCode,
		Code:       *envelope.Code,
		Message:    envelopeMessage(envelope.Message),
		Details:    string(body),
	}
}

// fileReader joins the bytes that Open read to find the content type with
// the rest of the body.
type fileReader struct {
	io.Reader
	closer io.Closer
}

// Close closes the response body.
func (r *fileReader) Close() error {
	return r.closer.Close()
}

// gzipBody decompresses a gzip response body.
type gzipBody struct {
	*gzip.Reader
	body io.ReadCloser
}

// Close closes the gzip reader and the response body.
func (g *gzipBody) Close() error {
	_ = g.Reader.Close()
	return g.body.Close()
}

// dispositionFileName returns the file name of a Content-Disposition header
// value, without a folder prefix. It returns "" when the value has no file
// name.
//
// The server sends the name in one of these forms:
//
//   - Percent-encoded UTF-8: filename=C1525-%E6%AD%A3%E9%9D%A2.jpg.
//   - Plain ASCII: filename=C1525.pdf.
//   - Raw GBK bytes with a folder prefix: filename=C2040/C2040-<GBK>.jpg.
//
// dispositionFileName also reads the RFC 5987 form, and it prefers that
// form to the filename parameter:
//
//	filename*=UTF-8''C1525-%E6%AD%A3%E9%9D%A2.jpg
//
// The standard library parser (mime.ParseMediaType) rejects the folder
// prefix and the raw bytes, so this function parses the value itself.
func dispositionFileName(header string) string {
	var plain, extended string
	for rest := header; rest != ""; {
		var param string
		param, rest = nextDispositionParam(rest)
		key, value, ok := strings.Cut(param, "=")
		if !ok {
			continue
		}
		value = unquoteParam(strings.TrimSpace(value))
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "filename*":
			if name, ok := decodeExtendedValue(value); ok && extended == "" {
				extended = name
			}
		case "filename":
			if plain == "" {
				plain = decodePercentName(value)
			}
		}
	}
	name := extended
	if name == "" {
		name = plain
	}
	return cleanFileName(name)
}

// nextDispositionParam returns the text before the first ";" that is not in
// a quoted string, and the text after that ";".
func nextDispositionParam(s string) (param, rest string) {
	inQuote, escaped := false, false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case escaped:
			escaped = false
		case inQuote && c == '\\':
			escaped = true
		case c == '"':
			inQuote = !inQuote
		case c == ';' && !inQuote:
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

// unquoteParam removes the quotes and the backslash escapes of a quoted
// string. It returns other values unchanged.
func unquoteParam(value string) string {
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return value
	}
	inner := value[1 : len(value)-1]
	var b strings.Builder
	escaped := false
	for i := 0; i < len(inner); i++ {
		if !escaped && inner[i] == '\\' {
			escaped = true
			continue
		}
		escaped = false
		b.WriteByte(inner[i])
	}
	return b.String()
}

// decodeExtendedValue decodes an RFC 5987 value: a character set, a
// language and the percent-encoded name, separated by single quotes. It
// accepts the UTF-8 and ISO-8859-1 character sets. It returns false for
// another character set or for a value that it cannot decode.
func decodeExtendedValue(value string) (string, bool) {
	parts := strings.SplitN(value, "'", 3)
	if len(parts) != 3 {
		return "", false
	}
	decoded, err := url.PathUnescape(parts[2])
	if err != nil {
		return "", false
	}
	switch strings.ToLower(strings.TrimSpace(parts[0])) {
	case "utf-8":
		if !utf8.ValidString(decoded) {
			return "", false
		}
		return decoded, true
	case "iso-8859-1":
		runes := make([]rune, 0, len(decoded))
		for i := 0; i < len(decoded); i++ {
			runes = append(runes, rune(decoded[i]))
		}
		return string(runes), true
	default:
		return "", false
	}
}

// decodePercentName decodes a percent-encoded UTF-8 file name. It returns
// the value unchanged when the value has no percent sign, when the decode
// fails, or when the result is not valid UTF-8.
func decodePercentName(value string) string {
	if !strings.Contains(value, "%") {
		return value
	}
	decoded, err := url.PathUnescape(value)
	if err != nil || !utf8.ValidString(decoded) {
		return value
	}
	return decoded
}

// cleanFileName removes the folder prefix, replaces the bytes that are not
// valid UTF-8 with "_", and removes control characters. It returns "" for
// "." and "..".
func cleanFileName(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ToValidUTF8(name, "_")
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "." || name == ".." {
		return ""
	}
	return name
}
