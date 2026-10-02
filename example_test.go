package jlcpcb_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/PatrickWalther/go-jlcpcb-parts"
)

func ExampleNewClient() {
	client := jlcpcb.NewClient()
	_ = client
}

func ExampleSearchService_Keyword() {
	client := jlcpcb.NewClient()
	resp, err := client.Search.Keyword(context.Background(), &jlcpcb.SearchRequest{
		Keyword:  "resistor",
		PageSize: 3,
	})
	if err != nil {
		fmt.Println("search error:", err)
		return
	}
	fmt.Println(len(resp.Products) > 0)
}

func ExampleProductService_Details() {
	client := jlcpcb.NewClient()
	product, err := client.Product.Details(context.Background(), "C3900982")
	if err != nil {
		fmt.Println("details error:", err)
		return
	}
	fmt.Println(product.ComponentCode)
}

func ExampleProductService_Detail() {
	client := jlcpcb.NewClient()
	detail, err := client.Product.Detail(context.Background(), "C1525")
	if errors.Is(err, jlcpcb.ErrNotFound) {
		fmt.Println("JLCPCB does not know this part")
		return
	}
	if err != nil {
		fmt.Println("detail error:", err)
		return
	}
	fmt.Println(detail.LCSCComponentID, detail.AssemblyProcess, detail.EncapsulationNumber)
}

func ExampleProductService_DetailsByIDs() {
	client := jlcpcb.NewClient()
	details, err := client.Product.DetailsByIDs(context.Background(), []int64{1877, 2392})
	if err != nil {
		fmt.Println("batch detail error:", err)
		return
	}
	for id, detail := range details {
		product := detail.Product()
		quote := product.PartsOrderQuote(5000)
		fmt.Println(id, detail.ComponentCode, quote.PreOrder, quote.MinQty)
	}
}

func ExampleComponentDetail_Product() {
	detail := jlcpcb.ComponentDetail{
		LCSCComponentID:      1877,
		ComponentCode:        "C1525",
		ParentCategory:       "Capacitors",
		LeafCategory:         "Multilayer Ceramic Capacitors MLCC - SMD/SMT",
		ComponentLibraryType: "base",
		Prices:               []jlcpcb.PriceBreak{{StartNumber: 1, EndNumber: -1, ProductPrice: 0.0045}},
	}

	product := detail.Product()
	parent, leaf := product.Category()
	fmt.Println(product.ComponentID, product.LibraryType())
	fmt.Println(parent, "/", leaf)
	// Output:
	// 1877 basic
	// Capacitors / Multilayer Ceramic Capacitors MLCC - SMD/SMT
}

func ExampleSearchService_Query() {
	client := jlcpcb.NewClient()
	resp, err := client.Search.Query(context.Background(), &jlcpcb.SearchRequest{
		Category: jlcpcb.Category{
			Parent: "Capacitors",
			Leaf:   "Multilayer Ceramic Capacitors MLCC - SMD/SMT",
		},
		Packages: []string{"0402"},
		AttributeFilters: []jlcpcb.AttributeFilter{
			{Name: "Capacitance", Values: []string{"100nF"}},
		},
		LibraryTypes:     []jlcpcb.ComponentType{jlcpcb.ComponentTypeBase},
		IncludePreferred: true,
		StockOnly:        true,
	})
	if err != nil {
		fmt.Println("query error:", err)
		return
	}
	fmt.Println(resp.TotalCount > 0)
}

func ExampleSearchService_Pages() {
	client := jlcpcb.NewClient()
	err := client.Search.Pages(context.Background(), &jlcpcb.SearchRequest{
		Keyword:  "SOIC-8",
		PageSize: 100,
	}, func(page *jlcpcb.SearchResponse) error {
		fmt.Printf("page %d of %d\n", page.Page, page.Pages)
		return nil
	})
	if err != nil {
		fmt.Println("pages error:", err)
	}
}

func ExampleProduct_PartsOrderQuote() {
	product := jlcpcb.Product{
		StockCount:        12,
		CanPresaleNumber:  8,
		MinPurchaseNum:    1,
		PreMinPurchaseNum: 8203,
		ComponentPrices:   []jlcpcb.PriceBreak{{StartNumber: 1, EndNumber: -1, ProductPrice: 0.0012}},
		BuyComponentPrices: []jlcpcb.PriceBreak{
			{StartNumber: 3000, EndNumber: -1, ProductPrice: 0.0010},
			{StartNumber: 1, EndNumber: 2999, ProductPrice: 0.0011},
		},
	}

	for _, qty := range []int{8, 9} {
		quote := product.PartsOrderQuote(qty)
		fmt.Printf("qty %d: pre-order %v, minimum %d, first tier %.4f USD\n",
			qty, quote.PreOrder, quote.MinQty, float64(quote.Ladder[0].ProductPrice))
	}
	// Output:
	// qty 8: pre-order false, minimum 1, first tier 0.0012 USD
	// qty 9: pre-order true, minimum 8203, first tier 0.0011 USD
}
