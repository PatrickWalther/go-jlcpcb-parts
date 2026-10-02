package jlcpcb

import (
	"net/url"
	"strings"
)

// fileDownloadPath is the path of the file download below the API root. The
// client adds the file access id.
const fileDownloadPath = "/file/downloadByFileSystemAccessId/"

// FileURL returns the stable download URL of a JLCPCB file access id, for
// example "https://jlcpcb.com/api/file/downloadByFileSystemAccessId/8552476004850417664".
// It returns "" when accessID is empty or has a character that is not a
// digit. FileURL ignores surrounding white space.
//
// The URL needs no headers. Unlike a signed URL, it has no signature and no
// expiry parameter. The same id gave the same bytes 45 minutes later, and an
// id that was 54 days old still gave a file. A byte comparison over more
// days was not done, so the durability over many days is not verified.
//
// The server sends a wrong Content-Type for images. Use FileService.Open,
// which finds the type from the bytes.
func FileURL(accessID string) string {
	id := strings.TrimSpace(accessID)
	if !isAccessID(id) {
		return ""
	}
	return defaultAPIRoot + fileDownloadPath + id
}

// isAccessID reports whether id is a file access id: one or more ASCII
// digits and nothing else.
func isAccessID(id string) bool {
	if id == "" {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] < '0' || id[i] > '9' {
			return false
		}
	}
	return true
}

// signedURLParams are the lower-case query parameter names of a signed URL.
// The JLCPCB OSS URLs use the "x-oss-" names. The other names are the older
// OSS and the S3 forms.
var signedURLParams = []string{
	"x-oss-signature",
	"x-oss-expires",
	"x-oss-credential",
	"x-oss-security-token",
	"signature",
	"ossaccesskeyid",
	"x-amz-signature",
	"x-amz-credential",
}

// IsSignedURL reports whether rawURL is a signed URL, which has an expiry
// time. It looks for a signature or expiry query parameter, for example
// "x-oss-signature". The match ignores case. It returns false for an empty
// URL and for a URL that it cannot parse.
//
// The "AccessIdUrl" fields of a Product and the "SignedURL" fields of a
// ComponentDetail hold signed URLs. They expire 30 or 60 minutes after the
// response. Do not store a URL for which IsSignedURL returns true.
func IsSignedURL(rawURL string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.RawQuery == "" {
		return false
	}
	for name := range u.Query() {
		for _, signed := range signedURLParams {
			if strings.EqualFold(name, signed) {
				return true
			}
		}
	}
	return false
}

// parseFileURL parses rawURL. It returns false when rawURL is not an http or
// https URL with a host and a file name. Some records send a folder URL
// without a file name, for example "https://assets.lcsc.com/images/lcsc/96x96/".
func parseFileURL(rawURL string) (*url.URL, bool) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return nil, false
	}
	if scheme := strings.ToLower(u.Scheme); scheme != "http" && scheme != "https" {
		return nil, false
	}
	if u.Path == "" || strings.HasSuffix(u.Path, "/") {
		return nil, false
	}
	return u, true
}

// hasFileName reports whether rawURL is an http or https URL with a file
// name.
func hasFileName(rawURL string) bool {
	_, ok := parseFileURL(rawURL)
	return ok
}

// isHostOf reports whether host is domain or a subdomain of domain. The
// match ignores case.
func isHostOf(host, domain string) bool {
	host = strings.ToLower(host)
	return host == domain || strings.HasSuffix(host, "."+domain)
}

// isLCSCMediaURL reports whether rawURL is an LCSC file URL without a
// signature, for example "https://assets.lcsc.com/images/lcsc/900x900/<name>.jpg".
// LCSC media URLs are not signed.
func isLCSCMediaURL(rawURL string) bool {
	u, ok := parseFileURL(rawURL)
	return ok && isHostOf(u.Hostname(), "lcsc.com") && !IsSignedURL(rawURL)
}

// fileURLAccessID returns the access id of a JLCPCB file URL, for example
// "https://jlcpcb.com/api/file/downloadByFileSystemAccessId/8588905303245541376".
// It returns "" for any other URL. Some records send such a URL in
// dataManualUrl and no id in dataManualFileAccessId.
func fileURLAccessID(rawURL string) string {
	u, ok := parseFileURL(rawURL)
	if !ok || !isHostOf(u.Hostname(), "jlcpcb.com") {
		return ""
	}
	id, ok := strings.CutPrefix(u.Path, "/api"+fileDownloadPath)
	if !ok || !isAccessID(id) {
		return ""
	}
	return id
}

// stableMediaURL returns the best URL of one file:
//
//  1. FileURL(accessID) when accessID is a file access id.
//  2. fallback when it is an LCSC media URL or a JLCPCB file URL.
//  3. signed when it is set. A signed URL expires.
//  4. "".
func stableMediaURL(accessID, fallback, signed string) string {
	if fileURL := FileURL(accessID); fileURL != "" {
		return fileURL
	}
	if id := fileURLAccessID(fallback); id != "" {
		return FileURL(id)
	}
	if isLCSCMediaURL(fallback) {
		return strings.TrimSpace(fallback)
	}
	if signed = strings.TrimSpace(signed); hasFileName(signed) {
		return signed
	}
	return ""
}

// stableDatasheetURL returns the best URL of the datasheet copy that JLCPCB
// hosts. It uses the access id of a JLCPCB file URL in dataManualURL when
// accessID is not set. It does not return an LCSC datasheet URL, because
// some LCSC datasheet URLs give an HTML viewer page, not a PDF.
func stableDatasheetURL(accessID, dataManualURL, signed string) string {
	if !isAccessID(strings.TrimSpace(accessID)) {
		accessID = fileURLAccessID(dataManualURL)
	}
	return stableMediaURL(accessID, "", signed)
}

// StableImageURL returns the best URL of the large image (900x900) of the
// part. The order is:
//
//  1. FileURL(ProductBigImageAccessId). This URL is not signed (see
//     FileURL).
//  2. ComponentImageUrl when it is an LCSC image URL or a JLCPCB file URL.
//     Some parts have no JLCPCB image and send an LCSC URL here (for
//     example C6186). An LCSC image URL is not signed. For a JLCPCB file
//     URL, the method returns FileURL of the access id in the URL.
//  3. ProductBigImageAccessIdUrl, a signed URL that expires 30 or 60 minutes
//     after the response.
//  4. "" when the part has no large image.
//
// Use IsSignedURL to find a signed result. Do not store a signed URL. The
// v2 search sends no file access ids today, so for a search row the result
// is usually a signed URL. ProductService.DetailsByIDs returns the access
// ids.
func (p *Product) StableImageURL() string {
	return stableMediaURL(p.ProductBigImageAccessId, p.ComponentImageUrl, p.ProductBigImageAccessIdUrl)
}

// StableThumbnailURL returns the best URL of the small image (96x96) of the
// part. It uses MinImageAccessId, MinImage and MinImageAccessIdUrl in the
// order of StableImageURL.
func (p *Product) StableThumbnailURL() string {
	return stableMediaURL(p.MinImageAccessId, p.MinImage, p.MinImageAccessIdUrl)
}

// StableDatasheetURL returns the best URL of the datasheet copy that
// JLCPCB hosts. The order is:
//
//  1. FileURL(DataManualFileAccessId). This URL is not signed (see
//     FileURL).
//  2. DataManualUrl when it is a JLCPCB file URL. Some records send the
//     file URL there and no access id.
//  3. DataManualFileAccessIdUrl, a signed URL that expires 30 or 60 minutes
//     after the response.
//  4. "" when JLCPCB hosts no copy.
//
// StableDatasheetURL does not return an LCSC datasheet URL from
// DataManualUrl, because a "www.lcsc.com/datasheet/" URL gives an HTML
// viewer page, not a PDF. DatasheetURLs returns all datasheet URLs.
func (p *Product) StableDatasheetURL() string {
	return stableDatasheetURL(p.DataManualFileAccessId, p.DataManualUrl, p.DataManualFileAccessIdUrl)
}

// StableImageURL returns the best URL of the large image (900x900) of the
// part. It uses ProductBigImageAccessID, ComponentImageURL and
// ProductBigImageSignedURL in the order of Product.StableImageURL.
func (d *ComponentDetail) StableImageURL() string {
	return stableMediaURL(d.ProductBigImageAccessID, d.ComponentImageURL, d.ProductBigImageSignedURL)
}

// StableThumbnailURL returns the best URL of the small image (96x96) of the
// part. It uses MinImageAccessID, MinImageURL and MinImageSignedURL in the
// order of Product.StableImageURL.
func (d *ComponentDetail) StableThumbnailURL() string {
	return stableMediaURL(d.MinImageAccessID, d.MinImageURL, d.MinImageSignedURL)
}

// StableDatasheetURL returns the best URL of the datasheet copy that
// JLCPCB hosts. It uses DataManualFileAccessID, DataManualURL and
// DataManualFileSignedURL in the order of Product.StableDatasheetURL.
func (d *ComponentDetail) StableDatasheetURL() string {
	return stableDatasheetURL(d.DataManualFileAccessID, d.DataManualURL, d.DataManualFileSignedURL)
}
