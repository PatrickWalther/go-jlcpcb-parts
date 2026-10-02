package jlcpcb

import (
	"encoding/json"
	"strings"
	"testing"
)

// compare_details_media.json holds 3 compareComponentDetails records of
// 2026-10-02 with only the identity, category and media fields. The signed
// URL parameters are replaced with fixed strings.
//
//   - C6186: no JLCPCB image, LCSC image URLs, a datasheet access id.
//   - C144256: LCSC folder URLs without a file name, a datasheet access id.
//   - C5213: image access ids, a JLCPCB file URL in dataManualUrl and no
//     datasheet access id.

const fileURLPrefix = "https://jlcpcb.com/api/file/downloadByFileSystemAccessId/"

func TestFileURL(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"8552476004850417664", fileURLPrefix + "8552476004850417664"},
		{" 8552476004850417664\n", fileURLPrefix + "8552476004850417664"},
		{"0", fileURLPrefix + "0"},
		{"", ""},
		{"   ", ""},
		{"abc", ""},
		{"855247600485041766x", ""},
		{"-1", ""},
		{"12 34", ""},
		{"1.5", ""},
		{"１２", ""}, // full-width digits
		{"8552476004850417664/../x", ""},
	}
	for _, tt := range tests {
		if got := FileURL(tt.id); got != tt.want {
			t.Errorf("FileURL(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestIsSignedURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://jlc-prod-smt.oss-eu-central-1.aliyuncs.com/smtComponentImageFile/1-C1525.jpg?x-oss-date=FIXTURE&x-oss-expires=FIXTURE&x-oss-signature=FIXTURE", true},
		{"https://jlc-prod-smt.oss-eu-central-1.aliyuncs.com/a.jpg?X-Oss-Signature=1", true},
		{"https://bucket.oss.example/a.pdf?OSSAccessKeyId=a&Expires=1&Signature=b", true},
		{"https://bucket.s3.example/a.pdf?X-Amz-Credential=a&X-Amz-Signature=b", true},
		{"https://assets.lcsc.com/images/lcsc/900x900/C6186_front.jpg", false},
		{"https://www.lcsc.com/datasheet/C1525.pdf?productCode=C1525", false},
		{fileURLPrefix + "8552476004850417664", false},
		{"", false},
		{"://bad", false},
	}
	for _, tt := range tests {
		if got := IsSignedURL(tt.url); got != tt.want {
			t.Errorf("IsSignedURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

// mediaWant holds the expected stable media URLs of one part.
type mediaWant struct {
	image, thumbnail, datasheet string
}

// mediaRecords decodes the compare records of a fixture by part code.
func mediaRecords(t *testing.T, fixture string) map[string]json.RawMessage {
	t.Helper()
	var resp struct {
		Data []struct {
			Detail json.RawMessage `json:"componentDetailVo"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loadFixture(t, fixture), &resp); err != nil {
		t.Fatalf("decode %s: %v", fixture, err)
	}
	records := make(map[string]json.RawMessage, len(resp.Data))
	for _, row := range resp.Data {
		var id struct {
			Code string `json:"componentCode"`
		}
		if err := json.Unmarshal(row.Detail, &id); err != nil {
			t.Fatalf("decode %s record: %v", fixture, err)
		}
		records[id.Code] = row.Detail
	}
	return records
}

// TestStableMediaURLsCompareRecords checks the stable URLs of live compare
// records. A ComponentDetail, its Product() and a Product decoded from the
// same JSON must give the same URLs.
func TestStableMediaURLsCompareRecords(t *testing.T) {
	records := mediaRecords(t, "compare_details.json")
	for code, raw := range mediaRecords(t, "compare_details_media.json") {
		records[code] = raw
	}

	tests := map[string]mediaWant{
		"C1525": {
			image:     fileURLPrefix + "8552476004850417664",
			thumbnail: fileURLPrefix + "8552476005450338304",
			datasheet: fileURLPrefix + "8579707269996871680",
		},
		"C6186": {
			image:     "https://assets.lcsc.com/images/lcsc/900x900/20221228_Advanced-Monolithic-Systems-AMS1117-3-3_C6186_front.jpg",
			thumbnail: "https://assets.lcsc.com/images/lcsc/96x96/20221228_Advanced-Monolithic-Systems-AMS1117-3-3_C6186_front.jpg",
			datasheet: fileURLPrefix + "8550724073479806976",
		},
		"C144256": {
			datasheet: fileURLPrefix + "8588881307011465216",
		},
		"C5213": {
			image:     fileURLPrefix + "8583412980671254528",
			thumbnail: fileURLPrefix + "8583412981593595904",
			datasheet: fileURLPrefix + "8588905303245541376",
		},
		"C5200613": {
			datasheet: fileURLPrefix + "8589835592130596864",
		},
	}

	for code, want := range tests {
		raw, ok := records[code]
		if !ok {
			t.Fatalf("no fixture record for %s", code)
		}
		var detail ComponentDetail
		if err := json.Unmarshal(raw, &detail); err != nil {
			t.Fatalf("%s: decode detail: %v", code, err)
		}
		var product Product
		if err := json.Unmarshal(raw, &product); err != nil {
			t.Fatalf("%s: decode product: %v", code, err)
		}
		mapped := detail.Product()

		got := []mediaWant{
			{detail.StableImageURL(), detail.StableThumbnailURL(), detail.StableDatasheetURL()},
			{product.StableImageURL(), product.StableThumbnailURL(), product.StableDatasheetURL()},
			{mapped.StableImageURL(), mapped.StableThumbnailURL(), mapped.StableDatasheetURL()},
		}
		for i, name := range []string{"ComponentDetail", "Product", "ComponentDetail.Product()"} {
			if got[i] != want {
				t.Errorf("%s %s stable URLs = %+v, want %+v", code, name, got[i], want)
			}
		}
		for _, u := range []string{want.image, want.thumbnail, want.datasheet} {
			if IsSignedURL(u) {
				t.Errorf("%s: stable URL %q is signed", code, u)
			}
		}
	}
}

// TestStableMediaURLsSearchRows checks v2 search rows. They have no file
// access ids, so the signed URLs are the only file URLs.
func TestStableMediaURLsSearchRows(t *testing.T) {
	var resp struct {
		Data struct {
			Page struct {
				List []Product `json:"list"`
			} `json:"componentPageInfo"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loadFixture(t, "search_v2_rows.json"), &resp); err != nil {
		t.Fatalf("decode search rows: %v", err)
	}
	rows := make(map[string]Product)
	for _, p := range resp.Data.Page.List {
		rows[p.ComponentCode] = p
	}

	c1525, ok := rows["C1525"]
	if !ok {
		t.Fatal("no C1525 row")
	}
	if got := c1525.StableImageURL(); got != c1525.ProductBigImageAccessIdUrl || !IsSignedURL(got) {
		t.Errorf("C1525 StableImageURL() = %q, want the signed large image URL", got)
	}
	if got := c1525.StableThumbnailURL(); got != c1525.MinImageAccessIdUrl || !IsSignedURL(got) {
		t.Errorf("C1525 StableThumbnailURL() = %q, want the signed small image URL", got)
	}
	// The LCSC datasheet URL is a viewer page, so the signed copy wins.
	if got := c1525.StableDatasheetURL(); got != c1525.DataManualFileAccessIdUrl || !IsSignedURL(got) {
		t.Errorf("C1525 StableDatasheetURL() = %q, want the signed datasheet URL", got)
	}

	// C17354991 has no image and only a signed datasheet URL.
	noImage, ok := rows["C17354991"]
	if !ok {
		t.Fatal("no C17354991 row")
	}
	if noImage.StableImageURL() != "" || noImage.StableThumbnailURL() != "" {
		t.Errorf("C17354991 image URLs = %q, %q, want empty", noImage.StableImageURL(), noImage.StableThumbnailURL())
	}
	if got := noImage.StableDatasheetURL(); got == "" || got != noImage.DataManualFileAccessIdUrl {
		t.Errorf("C17354991 StableDatasheetURL() = %q, want the signed datasheet URL", got)
	}
}

// TestStableMediaURLOrder checks the order and the URL checks of the stable
// URL methods.
func TestStableMediaURLOrder(t *testing.T) {
	const (
		signedBig  = "https://jlc-prod-smt.oss-eu-central-1.aliyuncs.com/smtComponentImageFile/1-big.jpg?x-oss-signature=FIXTURE"
		signedMin  = "https://jlc-prod-smt.oss-eu-central-1.aliyuncs.com/smtComponentImageFile/1-min.jpg?x-oss-signature=FIXTURE"
		signedPDF  = "https://jlc-prod-smt.oss-eu-central-1.aliyuncs.com/smtDataManualFile/1-C1.pdf?x-oss-signature=FIXTURE"
		lcscBig    = "https://assets.lcsc.com/images/lcsc/900x900/C1_front.jpg"
		lcscMin    = "http://assets.lcsc.com/images/lcsc/96x96/C1_front.jpg"
		lcscSheet  = "https://wmsc.lcsc.com/wmsc/upload/file/pdf/v2/lcsc/C1.pdf"
		accessID   = "8552476004850417664"
		accessURL  = fileURLPrefix + accessID
		otherImage = "https://example.com/C1_front.jpg"
	)

	tests := []struct {
		name    string
		product Product
		want    mediaWant
	}{
		{
			name: "access ids first",
			product: Product{
				ProductBigImageAccessId: " " + accessID + " ", MinImageAccessId: accessID, DataManualFileAccessId: accessID,
				ComponentImageUrl: lcscBig, MinImage: lcscMin, DataManualUrl: lcscSheet,
				ProductBigImageAccessIdUrl: signedBig, MinImageAccessIdUrl: signedMin, DataManualFileAccessIdUrl: signedPDF,
			},
			want: mediaWant{accessURL, accessURL, accessURL},
		},
		{
			name: "LCSC images before signed URLs",
			product: Product{
				ComponentImageUrl: " " + lcscBig, MinImage: lcscMin, DataManualUrl: lcscSheet,
				ProductBigImageAccessIdUrl: signedBig, MinImageAccessIdUrl: signedMin, DataManualFileAccessIdUrl: signedPDF,
			},
			want: mediaWant{lcscBig, lcscMin, signedPDF},
		},
		{
			name: "access id that is not a number",
			product: Product{
				ProductBigImageAccessId: "abc", MinImageAccessId: "12x", DataManualFileAccessId: "null",
				ProductBigImageAccessIdUrl: signedBig, MinImageAccessIdUrl: signedMin, DataManualFileAccessIdUrl: signedPDF,
			},
			want: mediaWant{signedBig, signedMin, signedPDF},
		},
		{
			name: "other hosts, folder URLs and signed LCSC URLs are not stable",
			product: Product{
				ComponentImageUrl:          otherImage,
				MinImage:                   "https://assets.lcsc.com/images/lcsc/96x96/",
				ProductBigImageAccessIdUrl: signedBig,
				MinImageAccessIdUrl:        signedMin,
			},
			want: mediaWant{signedBig, signedMin, ""},
		},
		{
			name: "look-alike LCSC hosts",
			product: Product{
				ComponentImageUrl: "https://assets.lcsc.com.example.net/a.jpg",
				MinImage:          "https://notlcsc.com/a.jpg",
			},
			want: mediaWant{},
		},
		{
			name: "signed LCSC URL",
			product: Product{
				ComponentImageUrl: lcscBig + "?x-oss-signature=FIXTURE",
			},
			want: mediaWant{},
		},
		{
			name: "JLCPCB file URL in dataManualUrl",
			product: Product{
				DataManualUrl:             "https://jlcpcb.com/api/file/downloadByFileSystemAccessId/8588905303245541376",
				DataManualFileAccessIdUrl: signedPDF,
			},
			want: mediaWant{datasheet: fileURLPrefix + "8588905303245541376"},
		},
		{
			name: "JLCPCB file URLs in componentImageUrl and minImage",
			product: Product{
				ComponentImageUrl:          "https://jlcpcb.com/api/file/downloadByFileSystemAccessId/123",
				MinImage:                   " https://JLCPCB.com/api/file/downloadByFileSystemAccessId/456?download=1 ",
				ProductBigImageAccessIdUrl: signedBig,
				MinImageAccessIdUrl:        signedMin,
			},
			want: mediaWant{image: fileURLPrefix + "123", thumbnail: fileURLPrefix + "456"},
		},
		{
			name: "other JLCPCB URL in dataManualUrl",
			product: Product{
				DataManualUrl: "https://jlcpcb.com/api/file/other/8588905303245541376",
			},
			want: mediaWant{},
		},
		{
			name:    "LCSC datasheet only",
			product: Product{DataManualUrl: lcscSheet},
			want:    mediaWant{},
		},
		{
			name:    "empty",
			product: Product{},
			want:    mediaWant{},
		},
	}

	for _, tt := range tests {
		p := tt.product
		got := mediaWant{p.StableImageURL(), p.StableThumbnailURL(), p.StableDatasheetURL()}
		if got != tt.want {
			t.Errorf("%s: product stable URLs = %+v, want %+v", tt.name, got, tt.want)
		}

		// A ComponentDetail with the same fields gives the same URLs.
		d := ComponentDetail{
			ProductBigImageAccessID:  p.ProductBigImageAccessId,
			MinImageAccessID:         p.MinImageAccessId,
			DataManualFileAccessID:   p.DataManualFileAccessId,
			ProductBigImageSignedURL: p.ProductBigImageAccessIdUrl,
			MinImageSignedURL:        p.MinImageAccessIdUrl,
			DataManualFileSignedURL:  p.DataManualFileAccessIdUrl,
			ComponentImageURL:        p.ComponentImageUrl,
			MinImageURL:              p.MinImage,
			DataManualURL:            p.DataManualUrl,
		}
		got = mediaWant{d.StableImageURL(), d.StableThumbnailURL(), d.StableDatasheetURL()}
		if got != tt.want {
			t.Errorf("%s: detail stable URLs = %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

// TestDatasheetURLsWithFileURLInDataManualURL checks that the access id URL
// and the same URL in dataManualUrl give one entry.
func TestDatasheetURLsWithFileURLInDataManualURL(t *testing.T) {
	p := Product{
		DataManualFileAccessId: "8588905303245541376",
		DataManualUrl:          fileURLPrefix + "8588905303245541376",
		DataManualOfficialLink: "http://www.ti.com/cn/lit/gpn/lm358",
	}
	got := p.DatasheetURLs()
	want := []string{fileURLPrefix + "8588905303245541376", "http://www.ti.com/cn/lit/gpn/lm358"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("DatasheetURLs() = %q, want %q", got, want)
	}
}
