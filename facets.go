package jlcpcb

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// filterComponentAttributePath is the path of the facet endpoint below the
// API root.
const filterComponentAttributePath = "/overseas-pcb-order/v1/componentSearch/filterComponentAttribute"

// Values of FacetRequest.FacetFor for the filter rows of the JLCPCB part
// pages. The part pages send these values. A parameter name, for example
// "Voltage Rating", is also a valid value.
const (
	// FacetForLibraryType selects the library type row (basic, extended and
	// preferred parts).
	FacetForLibraryType = "partsType"
	// FacetForStock selects the availability row (stock, pre-order and
	// consignment).
	FacetForStock = "stockType"
	// FacetForPCBA selects the PCBA type row.
	FacetForPCBA = "pcbaType"
	// FacetForInformation selects the datasheet and photo row.
	FacetForInformation = "information"
	// FacetForPackage selects the package row.
	FacetForPackage = "componentSpecificationList"
)

// FacetRequest selects the parts of a facet query. The fields work like the
// SearchRequest fields with the same names, with one difference: the facet
// endpoint takes numeric category ids, not category names.
//
// Get the category ids from ComponentDetail.ParentCategoryID and
// LeafCategoryID, from CategoryCount.ID, or from CategoryService.Info.
type FacetRequest struct {
	// Keyword is an optional search text.
	Keyword string
	// ParentID is the numeric id of the first-level category
	// (productTypeIdList), for example 2 for "Capacitors". 0 does not filter.
	ParentID int
	// LeafID is the numeric id of the leaf category (componentTypeIdList),
	// for example 2929. Set ParentID too, as the JLCPCB part pages do.
	// 0 does not filter.
	LeafID int
	// Packages keeps only parts with one of these packages
	// (componentSpecificationList), for example "0402".
	Packages []string
	// Brands keeps only parts of these manufacturers (componentBrandList).
	Brands []string
	// LibraryTypes keeps only parts of these library types
	// (orderLibraryTypeList).
	LibraryTypes []ComponentType
	// IncludePreferred adds preferred extended parts
	// (preferredComponentFlag). With LibraryTypes set to ComponentTypeBase,
	// the facets count parts that are basic or preferred.
	IncludePreferred bool
	// PresaleTypes keeps only parts of these availability classes
	// (presaleTypes).
	PresaleTypes []PresaleType
	// PCBA keeps only parts that a PCBA type accepts (pcbAType).
	PCBA PCBAFilter
	// HasDatasheet keeps only parts with a datasheet (dateSheet).
	HasDatasheet bool
	// Attributes keeps only parts with the given attribute values
	// (paramList). The values are exact strings: "100nF" matches, but
	// "0.1uF" does not. Use ParamFacet.Canonical to find the exact strings.
	Attributes []AttributeFilter
	// FacetFor removes one filter from the facet counts (nowCondition).
	// Use a parameter name or one of the FacetFor constants. The server
	// then counts every facet without that filter, so one call shows which
	// values exist for the row. Total stays filtered, and the library
	// counts become flags (see Facets.CountsAreFlags).
	FacetFor string
}

// Facets holds the part counts of a facet query. SearchService.Facets
// returns it.
type Facets struct {
	// Total is the number of parts that match all filters.
	Total int `json:"total"`
	// Counts holds the part counts by library type and other classes.
	Counts LibraryCounts `json:"counts"`
	// CountsAreFlags is true when FacetRequest.FacetFor is set. The server
	// then sends 1 for a class with parts and 0 for a class without parts,
	// in place of a count.
	CountsAreFlags bool `json:"countsAreFlags"`
	// Presale holds the part count of each availability class
	// (presaleTypeAggs). Parts without a class are not in the map.
	Presale map[PresaleType]int `json:"presale,omitempty"`
	// Categories is the category tree of the matching parts, with part
	// counts. The tree has the leaf categories when the server sends them.
	// The tree "Others" (id 35) holds parts without a real category. Skip
	// it to classify a part.
	Categories []CategoryCount `json:"categories,omitempty"`
	// Packages holds the part count of each package.
	Packages []Bucket `json:"packages,omitempty"`
	// Brands holds the part count of each manufacturer.
	Brands []Bucket `json:"brands,omitempty"`
	// Params holds the values of each attribute, with part counts.
	Params []ParamFacet `json:"params,omitempty"`
}

// LibraryCounts holds the part counts of a facet query by library type and
// other classes. The JSON tags are the wire names.
type LibraryCounts struct {
	Basic              int `json:"basePart"`          // Basic parts
	Preferred          int `json:"preferredPart"`     // Preferred extended parts
	Extended           int `json:"extendPart"`        // Extended parts, preferred parts included
	Economic           int `json:"economicPart"`      // Parts that Economic PCBA accepts
	Standard           int `json:"standardPart"`      // Parts that Standard PCBA accepts
	Datasheet          int `json:"dataSheetPart"`     // Parts with a datasheet
	Photo              int `json:"photo"`             // Parts with a photo
	MechanicalAssembly int `json:"assemblyComponent"` // Mechanical assembly parts
}

// Bucket is one value of a facet with its part count.
type Bucket struct {
	Value string `json:"value"` // Exact filter value (key)
	Name  string `json:"name"`  // Display name
	Count int    `json:"count"` // Number of matching parts
}

// ParamFacet holds the values of one attribute in a facet query.
type ParamFacet struct {
	// Name is the attribute name, for example "Capacitance".
	Name string `json:"name"`
	// Range is true for a numeric attribute (rangeFlag).
	Range bool `json:"range"`
	// Units lists the units of the attribute, smallest first (unitList).
	Units []string `json:"units,omitempty"`
	// UnitScale maps each unit to its factor to the base unit
	// (unitConversionMap), for example {"pF": 1, "nF": 1000, "uF": 1e6}.
	// The base unit has the factor 1. ParamValue.Norm uses the base unit.
	UnitScale map[string]float64 `json:"unitScale,omitempty"`
	// Values are the attribute values in the order of the server.
	Values []ParamValue `json:"values"`
}

// ParamValue is one value of an attribute in a facet query.
type ParamValue struct {
	// Value is the exact value string, for example "100nF". Use it in a
	// filter.
	Value string `json:"value"`
	// Count is the number of matching parts with this value.
	Count int `json:"count"`
	// DocCount is the raw count of the server. It counts a part twice for a
	// symmetric "±" value, for example "±10%". Count does not.
	DocCount int `json:"docCount"`
	// Norm is the value in the base unit of the attribute
	// (numericalNormValue), for example 100000 for "100nF" (pF). It is 0
	// for a range, for text and for some single values (see IntervalStart).
	Norm float64 `json:"norm"`
	// IntervalStart and IntervalEnd are the ends of a range in the base
	// unit, for example -40 and 125 for "-40℃~+125℃". Some single values,
	// for example "3.3V", have Norm 0 and the value in both ends.
	IntervalStart float64 `json:"intervalStart"`
	IntervalEnd   float64 `json:"intervalEnd"`
}

// Param returns the facet of the attribute name. The match ignores case.
func (f *Facets) Param(name string) (ParamFacet, bool) {
	name = strings.TrimSpace(name)
	for _, p := range f.Params {
		if strings.EqualFold(p.Name, name) {
			return p, true
		}
	}
	return ParamFacet{}, false
}

// Facets returns the part counts of the parts that req selects: library
// counts, availability classes, categories, packages, brands and attribute
// values. It sends one POST request to filterComponentAttribute on a cache
// miss. The cache keeps the answer for CacheConfig.FacetsTTL (15 minutes by
// default).
//
// Facets returns ErrInvalidRequest when req is nil or a category id is less
// than 0. The server rejects a body with a wrong shape with envelope code
// 101 (ErrRejected).
func (s *SearchService) Facets(ctx context.Context, req *FacetRequest) (*Facets, error) {
	if req == nil {
		return nil, fmt.Errorf("%w: facet request is nil", ErrInvalidRequest)
	}
	if req.ParentID < 0 || req.LeafID < 0 {
		return nil, fmt.Errorf("%w: category ids must not be less than 0", ErrInvalidRequest)
	}

	c := s.client
	body := newFacetRequestBody(req)
	cacheKey := cacheKeyForFacets(body)
	var cached Facets
	if c.cachedValue(cacheKey, &cached) {
		return &cached, nil
	}

	var resp facetsResponse
	if err := c.doAPI(ctx, http.MethodPost, filterComponentAttributePath, nil, body, &resp); err != nil {
		return nil, err
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("jlcpcb: facet response has no data")
	}

	facets := resp.Data.facets(body.NowCondition != "")
	c.cacheValue(cacheKey, facets, c.cacheConfig.facetsTTL())
	return facets, nil
}

// facetRequestBody is the body of filterComponentAttribute. The JLCPCB part
// pages send it in this shape.
type facetRequestBody struct {
	BaseQuery    facetBaseQuery `json:"baseQueryDto"`
	CatalogLevel int            `json:"catalogLevel"`
	NowCondition string         `json:"nowCondition"`
	ParamList    []facetParam   `json:"paramList"`
	QueryString  string         `json:"queryString,omitempty"`
}

// facetBaseQuery is the filter part of a facet request body. The fields with
// omitempty are optional.
type facetBaseQuery struct {
	ComponentBrandList         []string `json:"componentBrandList"`
	ComponentSpecificationList []string `json:"componentSpecificationList"`
	ComponentTypeIDList        []int    `json:"componentTypeIdList"`
	OrderLibraryTypeList       []string `json:"orderLibraryTypeList"`
	PackageTypeList            []string `json:"packageTypeList"`
	ProductTypeIDList          []int    `json:"productTypeIdList"`
	// The facet endpoint ignores componentLibTypes, but the part pages send
	// it next to orderLibraryTypeList.
	ComponentLibTypes      []string `json:"componentLibTypes,omitempty"`
	PreferredComponentFlag bool     `json:"preferredComponentFlag,omitempty"`
	PresaleTypes           []string `json:"presaleTypes,omitempty"`
	PCBAType               int      `json:"pcbAType,omitempty"`
	DateSheet              bool     `json:"dateSheet,omitempty"`
	FilterType             int      `json:"filterType,omitempty"`
	Keyword                string   `json:"keyword,omitempty"`
}

// facetParam is one attribute filter of a facet request body.
type facetParam struct {
	ParamName      string   `json:"paramName"`
	ParamValueList []string `json:"paramValueList"`
}

const (
	// facetFilterTypeKeyword is the filterType of a facet request with a
	// keyword.
	facetFilterTypeKeyword = 1

	// Catalog levels of a facet request. The part pages send level 2 for a
	// leaf category and level 1 for a keyword. Level 1 for a parent
	// category alone and level 0 without a category or keyword gave correct
	// counts in live requests.
	facetCatalogAll    = 0
	facetCatalogParent = 1
	facetCatalogLeaf   = 2
)

// newFacetRequestBody returns the facet request body of req.
func newFacetRequestBody(req *FacetRequest) facetRequestBody {
	base := facetBaseQuery{
		ComponentBrandList:         trimmedValues(req.Brands),
		ComponentSpecificationList: trimmedValues(req.Packages),
		ComponentTypeIDList:        []int{},
		OrderLibraryTypeList:       []string{},
		PackageTypeList:            []string{},
		ProductTypeIDList:          []int{},
		PreferredComponentFlag:     req.IncludePreferred,
		PresaleTypes:               normalizedPresaleTypes(req.PresaleTypes),
		DateSheet:                  req.HasDatasheet,
	}
	if req.LeafID > 0 {
		base.ComponentTypeIDList = []int{req.LeafID}
	}
	if req.ParentID > 0 {
		base.ProductTypeIDList = []int{req.ParentID}
	}
	if libraryTypes := normalizedLibraryTypes(req.LibraryTypes); len(libraryTypes) > 0 {
		base.OrderLibraryTypeList = libraryTypes
		base.ComponentLibTypes = libraryTypes
	}
	if req.PCBA > 0 {
		base.PCBAType = int(req.PCBA)
	}

	body := facetRequestBody{
		NowCondition: strings.TrimSpace(req.FacetFor),
		ParamList:    []facetParam{},
	}
	keyword := strings.TrimSpace(req.Keyword)
	if keyword != "" {
		base.FilterType = facetFilterTypeKeyword
		base.Keyword = keyword
		body.QueryString = keyword
	}
	switch {
	case req.LeafID > 0:
		body.CatalogLevel = facetCatalogLeaf
	case req.ParentID > 0 || keyword != "":
		body.CatalogLevel = facetCatalogParent
	default:
		body.CatalogLevel = facetCatalogAll
	}
	for _, attr := range attributeList(nil, req.Attributes) {
		for name, values := range attr {
			body.ParamList = append(body.ParamList, facetParam{ParamName: name, ParamValueList: values})
		}
	}
	body.BaseQuery = base
	return body
}

// trimmedValues returns the values without surrounding white space, empty
// values and duplicate values. It returns an empty, non-nil list when no
// value is left.
func trimmedValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !slices.Contains(out, value) {
			out = append(out, value)
		}
	}
	return out
}

// cacheKeyForFacets returns the cache key of a facet request body. The key
// covers every field that the request sends.
func cacheKeyForFacets(body facetRequestBody) string {
	serialized, err := json.Marshal(body)
	if err != nil {
		return fmt.Sprintf("facets:%+v", body)
	}
	hash := sha256.Sum256(serialized)
	return fmt.Sprintf("facets:%x", hash)
}

// facetsResponse is the response of filterComponentAttribute.
type facetsResponse struct {
	Data *facetsData `json:"data"`
}

// facetsData is the data object of a facet response. The server sends the
// attribute facets in paramList and paramRangeList for a category query,
// and in parentParamList and parentParamRangeList for some keyword queries.
// The range lists have the numeric fields.
type facetsData struct {
	Total                int             `json:"total"`
	ProductTypeList      []facetBucket   `json:"productTypeList"`
	ProductTypeAggs      []facetBucket   `json:"productTypeAggs"`
	Packages             []facetBucket   `json:"componentSpecificationList"`
	Brands               []facetBucket   `json:"componentBrandList"`
	PresaleTypeAggs      []facetBucket   `json:"presaleTypeAggs"`
	ParamList            []facetParamAgg `json:"paramList"`
	ParentParamList      []facetParamAgg `json:"parentParamList"`
	ParamRangeList       []facetParamAgg `json:"paramRangeList"`
	ParentParamRangeList []facetParamAgg `json:"parentParamRangeList"`
	LibraryCounts
}

// facetBucket is one bucket of a facet response.
type facetBucket struct {
	Key      string        `json:"key"`
	Name     string        `json:"name"`
	DocCount int           `json:"docCount"`
	SubAggs  []facetBucket `json:"subAggs"`
}

// facetParamAgg is one attribute or attribute value of a facet response. A
// JSON null decodes to the zero value.
type facetParamAgg struct {
	Key                string                 `json:"key"`
	Name               string                 `json:"name"`
	DocCount           int                    `json:"docCount"`
	RangeFlag          bool                   `json:"rangeFlag"`
	NumericalNormValue FlexFloat64            `json:"numericalNormValue"`
	IntervalStartValue FlexFloat64            `json:"intervalStartValue"`
	IntervalEndValue   FlexFloat64            `json:"intervalEndValue"`
	UnitConversionMap  map[string]FlexFloat64 `json:"unitConversionMap"`
	UnitList           []string               `json:"unitList"`
	SubAggs            []facetParamAgg        `json:"subAggs"`
}

// facets converts the data object. flags tells whether the request had a
// nowCondition.
func (d *facetsData) facets(flags bool) *Facets {
	out := &Facets{
		Total:          d.Total,
		Counts:         d.LibraryCounts,
		CountsAreFlags: flags,
		Packages:       facetBuckets(d.Packages),
		Brands:         facetBuckets(d.Brands),
	}

	for _, b := range d.PresaleTypeAggs {
		key := strings.TrimSpace(b.Key)
		if key == "" {
			continue
		}
		if out.Presale == nil {
			out.Presale = make(map[PresaleType]int)
		}
		out.Presale[PresaleType(key)] += b.DocCount
	}

	categories := d.ProductTypeAggs
	if len(categories) == 0 {
		categories = d.ProductTypeList
	}
	for _, parent := range categories {
		node := CategoryCount{ID: atoiOrZero(parent.Key), Name: bucketName(parent), Level: 1, Count: parent.DocCount}
		for _, leaf := range parent.SubAggs {
			node.Children = append(node.Children, CategoryCount{
				ID:       atoiOrZero(leaf.Key),
				ParentID: node.ID,
				Name:     bucketName(leaf),
				Level:    2,
				Count:    leaf.DocCount,
			})
		}
		out.Categories = append(out.Categories, node)
	}

	// Prefer the range lists, because they have the numeric fields. Add an
	// attribute of the plain lists only when no range list has it.
	seen := make(map[string]bool)
	for _, list := range [][]facetParamAgg{d.ParamRangeList, d.ParentParamRangeList, d.ParamList, d.ParentParamList} {
		for _, agg := range list {
			name := bucketNameOfParam(agg)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			out.Params = append(out.Params, agg.paramFacet(name))
		}
	}
	return out
}

// paramFacet converts one attribute of a facet response.
func (agg facetParamAgg) paramFacet(name string) ParamFacet {
	facet := ParamFacet{
		Name:   name,
		Range:  agg.RangeFlag,
		Values: make([]ParamValue, 0, len(agg.SubAggs)),
	}
	if len(agg.UnitList) > 0 {
		facet.Units = agg.UnitList
	}
	if len(agg.UnitConversionMap) > 0 {
		facet.UnitScale = make(map[string]float64, len(agg.UnitConversionMap))
		for unit, factor := range agg.UnitConversionMap {
			facet.UnitScale[unit] = float64(factor)
		}
	}
	for _, v := range agg.SubAggs {
		value := ParamValue{
			Value:         bucketNameOfParam(v),
			Count:         v.DocCount,
			DocCount:      v.DocCount,
			Norm:          float64(v.NumericalNormValue),
			IntervalStart: float64(v.IntervalStartValue),
			IntervalEnd:   float64(v.IntervalEndValue),
		}
		// Each value has one sub-bucket ("-") with the true part count.
		// The value count is twice this count for a symmetric "±" value.
		if len(v.SubAggs) > 0 {
			value.Count = 0
			for _, sub := range v.SubAggs {
				value.Count += sub.DocCount
			}
		}
		facet.Values = append(facet.Values, value)
	}
	return facet
}

func facetBuckets(buckets []facetBucket) []Bucket {
	if len(buckets) == 0 {
		return nil
	}
	out := make([]Bucket, 0, len(buckets))
	for _, b := range buckets {
		value := b.Key
		if value == "" {
			value = b.Name
		}
		out = append(out, Bucket{Value: value, Name: bucketName(b), Count: b.DocCount})
	}
	return out
}

func bucketName(b facetBucket) string {
	if b.Name != "" {
		return b.Name
	}
	return b.Key
}

func bucketNameOfParam(agg facetParamAgg) string {
	if agg.Key != "" {
		return agg.Key
	}
	return agg.Name
}

func atoiOrZero(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

// Canonical returns the values of the facet that have the same meaning as
// input, in the order of Values. It returns nil when no value matches. Use
// the result in AttributeFilter.Values or FacetRequest.Attributes, because
// the server matches exact strings only.
//
// A value matches when one of these conditions is true:
//
//   - The value is equal to input. The match ignores case.
//   - The value has the same number in the base unit as input. For example
//     "0.1uF" matches "100nF", and "-40°C~+125°C" matches "-40℃~+125℃" and
//     "-40℃~+125℃@(Tj)".
//
// Canonical reads input as a number with an optional unit of UnitScale
// ("0.1uF", "100 nF", "10k" for "10kΩ"), a range ("-40℃~+125℃") or a
// symmetric range ("±1%"). A number without a unit uses the base unit.
// Canonical ignores a condition after "@", for example "@(Tj)".
//
// For a value, Canonical uses Norm. When Norm is 0, it uses IntervalStart
// and IntervalEnd: equal ends are a single value (for example "3.3V"), and
// different ends are a range. A single value does not match a range, so
// "1%" does not match "±1%".
func (f ParamFacet) Canonical(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	want, numeric := f.parseQuantity(input)

	var out []string
	for _, v := range f.Values {
		if strings.EqualFold(v.Value, input) {
			out = append(out, v.Value)
			continue
		}
		if !numeric {
			continue
		}
		if got, ok := f.valueQuantity(v); ok && got.equal(want) {
			out = append(out, v.Value)
		}
	}
	return out
}

// quantity is a single value (lo == hi) or a range in the base unit.
type quantity struct {
	lo, hi float64
}

func (q quantity) equal(other quantity) bool {
	return sameNumber(q.lo, other.lo) && sameNumber(q.hi, other.hi)
}

// sameNumber compares two numbers with a relative tolerance of 1e-9.
func sameNumber(a, b float64) bool {
	if a == b {
		return true
	}
	return math.Abs(a-b) <= 1e-9*math.Max(math.Abs(a), math.Abs(b))
}

// valueQuantity returns the numeric meaning of a facet value.
func (f ParamFacet) valueQuantity(v ParamValue) (quantity, bool) {
	switch {
	case v.Norm != 0:
		return quantity{v.Norm, v.Norm}, true
	case v.IntervalStart != 0 || v.IntervalEnd != 0:
		return quantity{v.IntervalStart, v.IntervalEnd}, true
	}
	// The server sends 0 for text and for the value 0. Accept 0 only when
	// the value string is a number.
	q, ok := f.parseQuantity(v.Value)
	if !ok || q.lo != 0 || q.hi != 0 {
		return quantity{}, false
	}
	return q, true
}

var (
	// numberWithUnit matches a number and the text after it.
	numberWithUnit = regexp.MustCompile(`^([+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)(.*)$`)
	// ohmText matches the text "ohm" or "ohms" at the end of a unit.
	ohmText = regexp.MustCompile(`(?i)ohms?$`)
	// unitReplacer changes common spellings to the unit spellings of JLCPCB.
	unitReplacer = strings.NewReplacer(
		" ", "",
		"µ", "u", // micro sign
		"μ", "u", // Greek small letter mu
		"Ω", "Ω", // ohm sign to Greek capital letter omega
		"°C", "℃",
		"ºC", "℃",
		"+/-", "±",
	)
)

// parseQuantity reads a single value, a range "a~b" or a symmetric range
// "±a" in the base unit of the facet.
func (f ParamFacet) parseQuantity(input string) (quantity, bool) {
	s := unitReplacer.Replace(strings.TrimSpace(input))
	if i := strings.Index(s, "@"); i >= 0 {
		s = s[:i]
	}
	if rest, ok := strings.CutPrefix(s, "±"); ok {
		x, ok := f.parseScalar(rest, "")
		if !ok {
			return quantity{}, false
		}
		return quantity{-math.Abs(x), math.Abs(x)}, true
	}
	if lo, hi, ok := strings.Cut(s, "~"); ok {
		// A unit on one end applies to both ends: "1~15V".
		_, hiUnit, _ := splitNumber(hi)
		a, okA := f.parseScalar(lo, hiUnit)
		b, okB := f.parseScalar(hi, "")
		if !okA || !okB {
			return quantity{}, false
		}
		return quantity{a, b}, true
	}
	x, ok := f.parseScalar(s, "")
	if !ok {
		return quantity{}, false
	}
	return quantity{x, x}, true
}

// parseScalar reads a number with an optional unit and returns it in the
// base unit. defaultUnit applies when s has no unit.
func (f ParamFacet) parseScalar(s, defaultUnit string) (float64, bool) {
	num, unit, ok := splitNumber(s)
	if !ok {
		return 0, false
	}
	if unit == "" {
		unit = defaultUnit
	}
	scale, ok := f.unitFactor(unit)
	if !ok {
		return 0, false
	}
	return num * scale, true
}

// splitNumber splits s into a number and a unit.
func splitNumber(s string) (float64, string, bool) {
	m := numberWithUnit.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return 0, "", false
	}
	num, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		return 0, "", false
	}
	unit := ohmText.ReplaceAllString(strings.TrimSpace(m[2]), "Ω")
	return num, unit, true
}

// unitFactor returns the factor of unit to the base unit. An empty unit is
// the base unit. A unit prefix without the unit symbol, for example "k" for
// "kΩ", also works. A unique match that ignores case also works, but "m"
// (milli) never matches "M" (mega).
func (f ParamFacet) unitFactor(unit string) (float64, bool) {
	if unit == "" {
		return 1, true
	}
	if factor, ok := f.UnitScale[unit]; ok {
		return factor, true
	}
	symbol := commonUnitSuffix(f.UnitScale)
	if symbol != "" && !strings.HasSuffix(unit, symbol) {
		if factor, ok := f.UnitScale[unit+symbol]; ok {
			return factor, true
		}
	}

	var match string
	matches := 0
	for key := range f.UnitScale {
		if sameUnit(key, unit) || (symbol != "" && sameUnit(key, unit+symbol)) {
			match = key
			matches++
		}
	}
	if matches != 1 {
		return 0, false
	}
	return f.UnitScale[match], true
}

// commonUnitSuffix returns the longest common suffix of the units, for
// example "F" for pF, nF and uF. It returns "" for fewer than 2 units.
func commonUnitSuffix(scale map[string]float64) string {
	if len(scale) < 2 {
		return ""
	}
	var suffix string
	first := true
	for unit := range scale {
		if first {
			suffix = unit
			first = false
			continue
		}
		for !strings.HasSuffix(unit, suffix) {
			_, size := utf8.DecodeRuneInString(suffix)
			suffix = suffix[size:]
		}
	}
	return suffix
}

// sameUnit compares two units and ignores case, but "m" (milli) never
// matches "M" (mega).
func sameUnit(a, b string) bool {
	if a == b {
		return true
	}
	if !strings.EqualFold(a, b) {
		return false
	}
	ra, _ := utf8.DecodeRuneInString(a)
	rb, _ := utf8.DecodeRuneInString(b)
	if ra != rb && (ra == 'm' || ra == 'M') {
		return false
	}
	return true
}
