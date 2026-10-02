package jlcpcb

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
)

const (
	// calculateAttritionPath is the path of the attrition calculator below
	// the API root. The "/overseas-pcb-order/v1" prefix gives HTTP 404.
	calculateAttritionPath = "/overseas-core-platform/shoppingCart/smtGood/calculateAttrition"
	// calculateOrderQtyPath is the path of the order quantity calculator
	// below the API root.
	calculateOrderQtyPath = "/overseas-core-platform/shoppingCart/smtGood/calculateComponentOrderQty"
)

// DefaultWastageCoefficient is the wastage coefficient of the JLCPCB
// attrition rule. The JLCPCB config key
// "SYSTEM.smt_config.smt_wastage_coefficient" has this value. The link
// between this key and the calculators is inferred: the value gives the
// answers of the live calculators for all recorded rows.
const DefaultWastageCoefficient = 0.002

// AssemblySide is the assembly side of a PCBA order (assemblySide).
type AssemblySide string

const (
	// AssemblySideSingle is an assembly on one side of the board.
	AssemblySideSingle AssemblySide = "single"
	// AssemblySideBoth is an assembly on both sides of the board. It doubles
	// the attrition.
	AssemblySideBoth AssemblySide = "both"
)

// PlacementRow is one part line of a PCBA order, as the JLCPCB attrition
// and order quantity calculators use it. The JSON tags are the wire names
// of the calculators.
//
// Copy LossNumber, LeastPatchNumber and EncapsulationNumber from the
// Product or ComponentDetail of the part.
type PlacementRow struct {
	// Side is the assembly side. An empty Side means AssemblySideSingle.
	Side AssemblySide `json:"assemblySide"`
	// Boards is the number of assembled boards (pasteNumber).
	Boards int `json:"pasteNumber"`
	// PerBoard is the number of placements of the part on one board
	// (componentDesignator).
	PerBoard int `json:"componentDesignator"`
	// LossNumber is the base term of the attrition of the part.
	LossNumber int `json:"lossNumber"`
	// LeastPatchNumber is the minimum placement quantity of the part.
	LeastPatchNumber int `json:"leastPatchNumber"`
	// EncapsulationNumber is the reel or tube size of the part. Placements
	// above this quantity add wastage.
	EncapsulationNumber int `json:"encapsulationNumber"`
}

// Validate returns an error that wraps ErrInvalidRequest when JLCPCB cannot
// calculate the row: Boards or PerBoard is 0 or less, Side is not known, or
// LossNumber, LeastPatchNumber or EncapsulationNumber is less than 0.
//
// The calculators answer a bad row with 0 and no error. Thus
// AssemblyService validates each row before it sends a request.
func (r PlacementRow) Validate() error {
	switch {
	case r.Boards <= 0:
		return fmt.Errorf("%w: boards (pasteNumber) must be more than 0, got %d", ErrInvalidRequest, r.Boards)
	case r.PerBoard <= 0:
		return fmt.Errorf("%w: placements per board (componentDesignator) must be more than 0, got %d", ErrInvalidRequest, r.PerBoard)
	case r.LossNumber < 0 || r.LeastPatchNumber < 0 || r.EncapsulationNumber < 0:
		return fmt.Errorf("%w: loss number, least patch number and encapsulation number must not be less than 0", ErrInvalidRequest)
	}
	if _, ok := normalizedSide(r.Side); !ok {
		return fmt.Errorf("%w: unknown assembly side %q, want %q or %q", ErrInvalidRequest, r.Side, AssemblySideSingle, AssemblySideBoth)
	}
	return nil
}

// normalizedSide returns the wire value of side. An empty side is
// AssemblySideSingle. The match ignores case and surrounding white space.
func normalizedSide(side AssemblySide) (AssemblySide, bool) {
	switch strings.ToLower(strings.TrimSpace(string(side))) {
	case "", string(AssemblySideSingle):
		return AssemblySideSingle, true
	case string(AssemblySideBoth):
		return AssemblySideBoth, true
	default:
		return "", false
	}
}

// EstimateAttrition returns the extra parts that JLCPCB adds to a placement
// row for wastage. It computes the rule of the JLCPCB attrition calculator
// without a request:
//
//	need      = Boards × PerBoard
//	attrition = k × (LossNumber + floor(coef × max(0, need − EncapsulationNumber)))
//
// k is 2 for AssemblySideBoth and 1 for the other sides. The rule gives the
// answer of the live calculator for every recorded row (68 rows on
// 2026-10-02). A coef that is not a finite number above 0 uses
// DefaultWastageCoefficient.
//
// EstimateAttrition returns 0 when Boards or PerBoard is 0 or less. The live
// calculator also answers 0 for such a row. A negative LossNumber or
// EncapsulationNumber counts as 0.
func EstimateAttrition(r PlacementRow, coef float64) int {
	if r.Boards <= 0 || r.PerBoard <= 0 {
		return 0
	}
	if math.IsNaN(coef) || math.IsInf(coef, 0) || coef <= 0 {
		coef = DefaultWastageCoefficient
	}

	need := r.Boards * r.PerBoard
	excess := max(0, need-max(0, r.EncapsulationNumber))
	attrition := max(0, r.LossNumber) + floorProduct(coef, excess)
	if side, _ := normalizedSide(r.Side); side == AssemblySideBoth {
		attrition *= 2
	}
	return attrition
}

// EstimateOrderQty returns the part quantity that JLCPCB charges for a
// placement row. It computes the rule of the JLCPCB order quantity
// calculator without a request:
//
//	qty = max(Boards × PerBoard + EstimateAttrition(r, coef), LeastPatchNumber)
//
// The rule does not apply a minimum order quantity or a reel size. The
// JLCPCB parts pages raise the quantity to the minimum order quantity
// themselves.
//
// EstimateOrderQty returns 0 when Boards or PerBoard is 0 or less.
func EstimateOrderQty(r PlacementRow, coef float64) int {
	if r.Boards <= 0 || r.PerBoard <= 0 {
		return 0
	}
	return max(r.Boards*r.PerBoard+EstimateAttrition(r, coef), r.LeastPatchNumber)
}

// floorProduct returns floor(coef × n). It adds a small relative tolerance
// before the floor, because a binary float can give 2.9999999999999996 for
// 0.002 × 1500.
func floorProduct(coef float64, n int) int {
	v := coef * float64(n)
	return int(math.Floor(v + 1e-9*math.Max(1, math.Abs(v))))
}

// AssemblyService calls the JLCPCB PCBA calculators.
type AssemblyService service

// calculatorResponse is the response of a calculator.
type calculatorResponse struct {
	Data []int `json:"data"`
}

// Attrition returns the attrition (extra parts for wastage) of each row from
// the JLCPCB attrition calculator, in the order of rows. It sends one POST
// request. An empty row list returns an empty list and sends no request.
//
// Attrition validates each row first. A row that is not valid gives an
// error that wraps ErrInvalidRequest, and Attrition sends no request.
// EstimateAttrition gives the same values without a request.
func (s *AssemblyService) Attrition(ctx context.Context, rows []PlacementRow) ([]int, error) {
	return s.calculate(ctx, calculateAttritionPath, rows)
}

// OrderQuantities returns the part quantity that JLCPCB charges for each
// row (placements plus attrition, at least LeastPatchNumber) from the
// JLCPCB order quantity calculator, in the order of rows. It sends one POST
// request. An empty row list returns an empty list and sends no request.
//
// OrderQuantities validates each row first. A row that is not valid gives
// an error that wraps ErrInvalidRequest, and OrderQuantities sends no
// request. EstimateOrderQty gives the same values without a request.
func (s *AssemblyService) OrderQuantities(ctx context.Context, rows []PlacementRow) ([]int, error) {
	return s.calculate(ctx, calculateOrderQtyPath, rows)
}

// calculate validates rows, sends them to the calculator at path and checks
// that the answer has one value for each row.
func (s *AssemblyService) calculate(ctx context.Context, path string, rows []PlacementRow) ([]int, error) {
	if len(rows) == 0 {
		return []int{}, nil
	}

	body := make([]PlacementRow, len(rows))
	for i, row := range rows {
		if err := row.Validate(); err != nil {
			return nil, fmt.Errorf("%w (row %d)", err, i)
		}
		row.Side, _ = normalizedSide(row.Side)
		body[i] = row
	}

	var resp calculatorResponse
	if err := s.client.doAPI(ctx, http.MethodPost, path, nil, body, &resp); err != nil {
		return nil, err
	}
	if len(resp.Data) != len(rows) {
		return nil, fmt.Errorf("jlcpcb: calculator returned %d values for %d rows", len(resp.Data), len(rows))
	}
	return resp.Data, nil
}
