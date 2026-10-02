package jlcpcb

import (
	"reflect"
	"slices"
	"testing"
)

func TestProductPartsOrderQuoteFixtureRows(t *testing.T) {
	products := decodeFixture(t, "search_v2_rows.json")
	tests := []struct {
		name     string
		code     string
		qty      int
		preOrder bool
		minQty   int
	}{
		// C1525: canPresaleNumber 15731372, minPurchaseNum 1, preMinPurchaseNum 2654.
		{"order from stock", "C1525", 100, false, 1},
		{"order at the stock limit", "C1525", 15731372, false, 1},
		{"order above the stock limit", "C1525", 15731373, true, 2654},
		// C89140: stock 12, canPresaleNumber 8, minPurchaseNum 1, preMinPurchaseNum 8203.
		{"small limit, order from stock", "C89140", 8, false, 1},
		{"small limit, pre-order", "C89140", 9, true, 8203},
		// C17354991: canPresaleNumber -3654, minPurchaseNum 2201, preMinPurchaseNum 2201.
		{"negative limit", "C17354991", 2201, true, 2201},
		// C2961140: canPresaleNumber 0, minPurchaseNum 6, preMinPurchaseNum 6.
		{"zero limit", "C2961140", 6, true, 6},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			product, ok := products[tt.code]
			if !ok {
				t.Fatalf("fixture has no product %s", tt.code)
			}
			quote := product.PartsOrderQuote(tt.qty)
			if quote.PreOrder != tt.preOrder || quote.MinQty != tt.minQty {
				t.Errorf("PartsOrderQuote(%d) = pre-order %v, min %d; want %v, %d",
					tt.qty, quote.PreOrder, quote.MinQty, tt.preOrder, tt.minQty)
			}

			wantLadder := product.SortedComponentPrices()
			if tt.preOrder {
				wantLadder = product.SortedBuyComponentPrices()
			}
			if !reflect.DeepEqual(quote.Ladder, wantLadder) {
				t.Errorf("Ladder = %+v, want %+v", quote.Ladder, wantLadder)
			}
			if !slices.IsSortedFunc(quote.Ladder, func(a, b PriceBreak) int { return a.StartNumber - b.StartNumber }) {
				t.Errorf("Ladder is not sorted: %v", startNumbers(quote.Ladder))
			}
		})
	}
}

// TestProductPartsOrderQuoteBuyLadder checks that a pre-order uses the buy
// ladder. The C1525 buy ladder is unsorted and cheaper than the stock ladder.
func TestProductPartsOrderQuoteBuyLadder(t *testing.T) {
	product := decodeFixture(t, "search_v2_rows.json")["C1525"]
	rawBuy := startNumbers(product.BuyComponentPrices)

	quote := product.PartsOrderQuote(20_000_000)
	if got, want := startNumbers(quote.Ladder), []int{1, 1000, 3000, 10000, 50000, 100000}; !reflect.DeepEqual(got, want) {
		t.Errorf("pre-order ladder start numbers = %v, want %v", got, want)
	}
	if got := float64(quote.Ladder[0].ProductPrice); got != 0.0043 {
		t.Errorf("pre-order tier 1 price = %v, want the buy price 0.0043", got)
	}
	if got := startNumbers(product.BuyComponentPrices); !reflect.DeepEqual(got, rawBuy) {
		t.Errorf("PartsOrderQuote changed BuyComponentPrices to %v", got)
	}

	stock := product.PartsOrderQuote(10)
	if got := float64(stock.Ladder[0].ProductPrice); got != 0.0045 {
		t.Errorf("stock tier 1 price = %v, want the stock price 0.0045", got)
	}
}

func TestProductPartsOrderQuoteRules(t *testing.T) {
	ladder := []PriceBreak{{StartNumber: 10, EndNumber: -1, ProductPrice: 0.5}, {StartNumber: 1, EndNumber: 9, ProductPrice: 0.6}}
	buyLadder := []PriceBreak{{StartNumber: 1, EndNumber: -1, ProductPrice: 0.55}}
	tests := []struct {
		name     string
		product  Product
		qty      int
		preOrder bool
		minQty   int
		ladder   []PriceBreak
	}{
		{
			name:    "limit below the minimum purchase, order from stock",
			product: Product{CanPresaleNumber: 5, MinPurchaseNum: 10, PreMinPurchaseNum: 20, ComponentPrices: ladder},
			qty:     3,
			minQty:  20,
			ladder:  SortPriceBreaks(ladder),
		},
		{
			name:     "limit below the minimum purchase, pre-order",
			product:  Product{CanPresaleNumber: 5, MinPurchaseNum: 10, PreMinPurchaseNum: 20, BuyComponentPrices: buyLadder},
			qty:      20,
			preOrder: true,
			minQty:   20,
			ladder:   buyLadder,
		},
		{
			name:     "pre-order minimum below the purchase minimum",
			product:  Product{CanPresaleNumber: 0, MinPurchaseNum: 30, PreMinPurchaseNum: 7},
			qty:      30,
			preOrder: true,
			minQty:   30,
		},
		{
			name:    "quantity below 1 counts as 1",
			product: Product{CanPresaleNumber: 1, MinPurchaseNum: 1, PreMinPurchaseNum: 50, ComponentPrices: ladder},
			qty:     0,
			minQty:  1,
			ladder:  SortPriceBreaks(ladder),
		},
		{
			name:     "no quantities in the row",
			product:  Product{},
			qty:      1,
			preOrder: true,
			minQty:   1,
		},
		{
			name:     "pre-order without a buy ladder",
			product:  Product{CanPresaleNumber: 0, MinPurchaseNum: 4, PreMinPurchaseNum: 4, ComponentPrices: ladder},
			qty:      4,
			preOrder: true,
			minQty:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quote := tt.product.PartsOrderQuote(tt.qty)
			want := PartsOrderQuote{PreOrder: tt.preOrder, MinQty: tt.minQty, Ladder: tt.ladder}
			if !reflect.DeepEqual(quote, want) {
				t.Errorf("PartsOrderQuote(%d) = %+v, want %+v", tt.qty, quote, want)
			}
		})
	}
}
