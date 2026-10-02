package jlcpcb

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// calculatorCase is one recorded calculator call: the rows of the request
// body and the answer of the live calculator.
//
// testdata/calculator_cases.json holds every recorded call of 2026-10-02:
// calculateAttrition and calculateComponentOrderQty on jlcpcb.com and
// cart.jlcpcb.com. Two calls send rows with a wrong shape. The calculator
// answers them as if the missing fields were 0.
type calculatorCase struct {
	Source     string         `json:"source"`
	Calculator string         `json:"calculator"` // "attrition" or "orderQty"
	Rows       []PlacementRow `json:"rows"`
	Answer     []int          `json:"answer"`
}

func loadCalculatorCases(t *testing.T) []calculatorCase {
	t.Helper()
	var cases []calculatorCase
	if err := json.Unmarshal(loadFixture(t, "calculator_cases.json"), &cases); err != nil {
		t.Fatalf("decode calculator cases: %v", err)
	}
	return cases
}

// referenceRow is the J6 reference row. The live attrition calculator
// answers 190 for it, and the order quantity calculator answers 100190.
var referenceRow = PlacementRow{
	Side:                AssemblySideSingle,
	Boards:              1000,
	PerBoard:            100,
	LossNumber:          10,
	LeastPatchNumber:    20,
	EncapsulationNumber: 10000,
}

func TestEstimateMatchesRecordedCalculators(t *testing.T) {
	cases := loadCalculatorCases(t)
	rows := 0
	for _, tc := range cases {
		if len(tc.Rows) != len(tc.Answer) {
			t.Fatalf("%s: %d rows and %d answers", tc.Source, len(tc.Rows), len(tc.Answer))
		}
		estimate := EstimateAttrition
		if tc.Calculator == "orderQty" {
			estimate = EstimateOrderQty
		}
		for i, row := range tc.Rows {
			rows++
			for _, coef := range []float64{DefaultWastageCoefficient, 0} {
				if got := estimate(row, coef); got != tc.Answer[i] {
					t.Errorf("%s %s row %d %+v (coef %g) = %d, want %d", tc.Source, tc.Calculator, i, row, coef, got, tc.Answer[i])
				}
			}
		}
	}
	if len(cases) != 14 || rows != 68 {
		t.Errorf("fixture has %d calls with %d rows, want 14 calls with 68 rows", len(cases), rows)
	}
}

func TestEstimateAttritionRules(t *testing.T) {
	tests := []struct {
		name string
		row  PlacementRow
		coef float64
		want int
	}{
		{name: "reference row", row: referenceRow, want: 190},
		{name: "no boards", row: PlacementRow{Boards: 0, PerBoard: 10, LossNumber: 10}, want: 0},
		{name: "no placements", row: PlacementRow{Boards: 10, PerBoard: -1, LossNumber: 10}, want: 0},
		{name: "need below the reel adds only the loss number", row: PlacementRow{Boards: 5, PerBoard: 4, LossNumber: 10, EncapsulationNumber: 10000}, want: 10},
		{name: "float product 0.002 x 1500 floors to 3", row: PlacementRow{Boards: 1500, PerBoard: 1}, want: 3},
		{name: "float product 0.002 x 3500 floors to 7", row: PlacementRow{Boards: 3500, PerBoard: 1}, want: 7},
		{name: "both sides double after the floor", row: PlacementRow{Side: AssemblySideBoth, Boards: 10300, PerBoard: 1, EncapsulationNumber: 10000}, want: 0},
		{name: "both sides double the loss number", row: PlacementRow{Side: "BOTH", Boards: 1, PerBoard: 1, LossNumber: 3}, want: 6},
		{name: "empty side is single", row: PlacementRow{Boards: 1, PerBoard: 1, LossNumber: 3}, want: 3},
		{name: "negative loss number counts as 0", row: PlacementRow{Boards: 1000, PerBoard: 1, LossNumber: -5}, want: 2},
		{name: "negative reel size counts as 0", row: PlacementRow{Boards: 1000, PerBoard: 1, EncapsulationNumber: -500}, want: 2},
		{name: "custom coefficient", row: PlacementRow{Boards: 1000, PerBoard: 1}, coef: 0.01, want: 10},
		{name: "NaN coefficient uses the default", row: PlacementRow{Boards: 1000, PerBoard: 1}, coef: math.NaN(), want: 2},
		{name: "negative coefficient uses the default", row: PlacementRow{Boards: 1000, PerBoard: 1}, coef: -1, want: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EstimateAttrition(tt.row, tt.coef); got != tt.want {
				t.Errorf("EstimateAttrition(%+v, %g) = %d, want %d", tt.row, tt.coef, got, tt.want)
			}
		})
	}
}

func TestEstimateOrderQty(t *testing.T) {
	tests := []struct {
		name string
		row  PlacementRow
		want int
	}{
		{name: "reference row", row: referenceRow, want: 100190},
		{name: "least patch number is the minimum", row: PlacementRow{Boards: 1, PerBoard: 1, LossNumber: 1, LeastPatchNumber: 50}, want: 50},
		{name: "no boards", row: PlacementRow{PerBoard: 1, LeastPatchNumber: 50}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EstimateOrderQty(tt.row, DefaultWastageCoefficient); got != tt.want {
				t.Errorf("EstimateOrderQty(%+v) = %d, want %d", tt.row, got, tt.want)
			}
		})
	}
}

func TestPlacementRowValidate(t *testing.T) {
	tests := []struct {
		name    string
		row     PlacementRow
		wantErr bool
	}{
		{name: "reference row", row: referenceRow},
		{name: "empty side", row: PlacementRow{Boards: 1, PerBoard: 1}},
		{name: "side in upper case", row: PlacementRow{Side: " Both ", Boards: 1, PerBoard: 1}},
		{name: "no boards", row: PlacementRow{PerBoard: 1}, wantErr: true},
		{name: "no placements", row: PlacementRow{Boards: 1}, wantErr: true},
		{name: "unknown side", row: PlacementRow{Side: "top", Boards: 1, PerBoard: 1}, wantErr: true},
		{name: "negative loss number", row: PlacementRow{Boards: 1, PerBoard: 1, LossNumber: -1}, wantErr: true},
		{name: "negative least patch number", row: PlacementRow{Boards: 1, PerBoard: 1, LeastPatchNumber: -1}, wantErr: true},
		{name: "negative reel size", row: PlacementRow{Boards: 1, PerBoard: 1, EncapsulationNumber: -1}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.row.Validate()
			if tt.wantErr != (err != nil) {
				t.Fatalf("Validate() = %v, want error %v", err, tt.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidRequest) {
				t.Errorf("Validate() = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestAssemblyAttritionRequest(t *testing.T) {
	rec := fixedAPIRecorder(t, []byte(`{"success":true,"code":200,"message":null,"errorCode":null,"data":[190,6]}`))

	rows := []PlacementRow{referenceRow, {Side: "Both", Boards: 1, PerBoard: 1, LossNumber: 3}}
	got, err := rec.client().Assembly.Attrition(context.Background(), rows)
	if err != nil {
		t.Fatalf("attrition failed: %v", err)
	}
	if !reflect.DeepEqual(got, []int{190, 6}) {
		t.Errorf("attrition = %v, want [190 6]", got)
	}

	requests := rec.all()
	if len(requests) != 1 {
		t.Fatalf("server got %d requests, want 1", len(requests))
	}
	req := requests[0]
	if req.Method != http.MethodPost || req.Path != calculateAttritionPath || req.ContentType != "application/json" {
		t.Errorf("request = %s %s (%s), want POST %s (application/json)", req.Method, req.Path, req.ContentType, calculateAttritionPath)
	}
	// The body uses the wire names of the calculator, and the side is
	// normalized.
	want := `[{"assemblySide":"single","pasteNumber":1000,"componentDesignator":100,"lossNumber":10,"leastPatchNumber":20,"encapsulationNumber":10000},` +
		`{"assemblySide":"both","pasteNumber":1,"componentDesignator":1,"lossNumber":3,"leastPatchNumber":0,"encapsulationNumber":0}]`
	if string(req.Body) != want {
		t.Errorf("body = %s\nwant   %s", req.Body, want)
	}
	if rows[1].Side != "Both" {
		t.Errorf("Attrition changed the side of the caller row to %q", rows[1].Side)
	}
}

func TestAssemblyOrderQuantitiesRequest(t *testing.T) {
	cases := loadCalculatorCases(t)
	var tc calculatorCase
	for _, c := range cases {
		if c.Source == "pcba-v09" {
			tc = c
		}
	}
	if tc.Calculator != "orderQty" {
		t.Fatalf("fixture has no order quantity call pcba-v09")
	}
	answer, _ := json.Marshal(map[string]interface{}{"success": true, "code": 200, "data": tc.Answer})
	rec := fixedAPIRecorder(t, answer)

	got, err := rec.client().Assembly.OrderQuantities(context.Background(), tc.Rows)
	if err != nil {
		t.Fatalf("order quantities failed: %v", err)
	}
	if !reflect.DeepEqual(got, tc.Answer) {
		t.Errorf("order quantities = %v, want %v", got, tc.Answer)
	}
	requests := rec.all()
	if len(requests) != 1 || requests[0].Path != calculateOrderQtyPath {
		t.Fatalf("requests = %+v, want 1 request to %s", requests, calculateOrderQtyPath)
	}
	var sent []PlacementRow
	if err := json.Unmarshal(requests[0].Body, &sent); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !reflect.DeepEqual(sent, tc.Rows) {
		t.Errorf("sent rows = %+v, want %+v", sent, tc.Rows)
	}
}

func TestAssemblyInvalidRowSendsNoRequest(t *testing.T) {
	rec := fixedAPIRecorder(t, []byte(`{"code":200,"data":[0]}`))
	client := rec.client()

	rows := []PlacementRow{referenceRow, {Boards: 0, PerBoard: 4}}
	for name, call := range map[string]func(context.Context, []PlacementRow) ([]int, error){
		"Attrition":       client.Assembly.Attrition,
		"OrderQuantities": client.Assembly.OrderQuantities,
	} {
		got, err := call(context.Background(), rows)
		if !errors.Is(err, ErrInvalidRequest) || got != nil {
			t.Errorf("%s = %v, %v, want ErrInvalidRequest", name, got, err)
		}
		if err != nil && !strings.Contains(err.Error(), "row 1") {
			t.Errorf("%s error %q does not name row 1", name, err)
		}
	}
	if n := len(rec.all()); n != 0 {
		t.Errorf("server got %d requests, want 0", n)
	}
}

func TestAssemblyEmptyRows(t *testing.T) {
	rec := fixedAPIRecorder(t, []byte(`{"code":200,"data":[]}`))
	got, err := rec.client().Assembly.Attrition(context.Background(), nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("Attrition(nil) = %v, %v, want an empty list", got, err)
	}
	if n := len(rec.all()); n != 0 {
		t.Errorf("server got %d requests, want 0", n)
	}
}

func TestAssemblyAnswerLengthMismatch(t *testing.T) {
	// The calculator answers [0] for a body with a wrong shape.
	rec := fixedAPIRecorder(t, []byte(`{"code":200,"data":[0]}`))
	got, err := rec.client().Assembly.Attrition(context.Background(), []PlacementRow{referenceRow, referenceRow})
	if err == nil || got != nil {
		t.Fatalf("Attrition = %v, %v, want an error", got, err)
	}
}

func TestAssemblyRejectedIsNotRetried(t *testing.T) {
	rec := fixedAPIRecorder(t, loadFixture(t, "search_v2_rejected.json"))
	client := rec.client(WithRetryConfig(RetryConfig{MaxRetries: 3, InitialBackoff: 1, MaxBackoff: 1, BackoffMultiplier: 1}))

	if _, err := client.Assembly.Attrition(context.Background(), []PlacementRow{referenceRow}); !errors.Is(err, ErrRejected) {
		t.Fatalf("error = %v, want ErrRejected", err)
	}
	if n := len(rec.all()); n != 1 {
		t.Errorf("server got %d requests, want 1", n)
	}
}
