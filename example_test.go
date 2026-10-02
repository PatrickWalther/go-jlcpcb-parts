package jlcpcb_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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
		quote := detail.Product().PartsOrderQuote(5000)
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

func ExampleEstimateAttrition() {
	row := jlcpcb.PlacementRow{
		Side:                jlcpcb.AssemblySideSingle,
		Boards:              1000,
		PerBoard:            100,
		LossNumber:          10,
		LeastPatchNumber:    20,
		EncapsulationNumber: 10000,
	}
	fmt.Println(jlcpcb.EstimateAttrition(row, jlcpcb.DefaultWastageCoefficient))
	fmt.Println(jlcpcb.EstimateOrderQty(row, jlcpcb.DefaultWastageCoefficient))
	// Output:
	// 190
	// 100190
}

func ExampleAssemblyService_OrderQuantities() {
	client := jlcpcb.NewClient()
	qty, err := client.Assembly.OrderQuantities(context.Background(), []jlcpcb.PlacementRow{
		{Boards: 50, PerBoard: 4, LossNumber: 10, LeastPatchNumber: 20, EncapsulationNumber: 10000},
		{Side: jlcpcb.AssemblySideBoth, Boards: 50, PerBoard: 2, LossNumber: 3, LeastPatchNumber: 5, EncapsulationNumber: 4000},
	})
	if err != nil {
		fmt.Println("calculator error:", err)
		return
	}
	fmt.Println(qty)
}

func ExampleSearchService_Facets() {
	client := jlcpcb.NewClient()
	ctx := context.Background()

	// The numeric category ids come from a detail record or a category tree.
	facets, err := client.Search.Facets(ctx, &jlcpcb.FacetRequest{
		ParentID: 2,
		LeafID:   2929,
		Packages: []string{"0402"},
		AttributeFilters: []jlcpcb.AttributeFilter{
			{Name: "Voltage Rating", Values: []string{"16V"}},
			{Name: "Capacitance", Values: []string{"100nF"}},
		},
	})
	if err != nil {
		fmt.Println("facets error:", err)
		return
	}
	fmt.Println(facets.Total, facets.Counts.Basic, facets.Presale[jlcpcb.PresaleTypeStock])

	// The server matches exact strings. Find the strings for "0.1uF".
	if capacitance, ok := facets.Param("Capacitance"); ok {
		fmt.Println(capacitance.Canonical("0.1uF"))
	}
}

func ExampleParamFacet_Canonical() {
	capacitance := jlcpcb.ParamFacet{
		Name:      "Capacitance",
		Range:     true,
		UnitScale: map[string]float64{"pF": 1, "nF": 1e3, "uF": 1e6},
		Values: []jlcpcb.ParamValue{
			{Value: "10nF", Norm: 1e4},
			{Value: "100nF", Norm: 1e5},
			{Value: "1uF", Norm: 1e6},
		},
	}
	fmt.Println(capacitance.Canonical("0.1uF"))
	fmt.Println(capacitance.Canonical("100000pF"))
	fmt.Println(capacitance.Canonical("1000n"))
	// Output:
	// [100nF]
	// [100nF]
	// [1uF]
}

func ExampleCategoryService_Info() {
	client := jlcpcb.NewClient()
	info, err := client.Category.Info(context.Background(), 2929)
	if err != nil {
		fmt.Println("category error:", err)
		return
	}
	resp, err := client.Search.Query(context.Background(), &jlcpcb.SearchRequest{
		Category: info.Category(),
		PageSize: 10,
	})
	if err != nil {
		fmt.Println("query error:", err)
		return
	}
	fmt.Println(info.ParentName, resp.TotalCount > 0)
}

func ExampleFileURL() {
	fmt.Println(jlcpcb.FileURL("8552476004850417664"))
	fmt.Println(jlcpcb.FileURL("not-an-id") == "")
	// Output:
	// https://jlcpcb.com/api/file/downloadByFileSystemAccessId/8552476004850417664
	// true
}

func ExampleProduct_StableImageURL() {
	product := jlcpcb.Product{
		ComponentCode:              "C1525",
		ProductBigImageAccessId:    "8552476004850417664",
		ProductBigImageAccessIdUrl: "https://jlc-prod-smt.oss-eu-central-1.aliyuncs.com/smtComponentImageFile/C1525.jpg?x-oss-expires=1800&x-oss-signature=abc",
		MinImage:                   "https://assets.lcsc.com/images/lcsc/96x96/C1525_front.jpg",
	}

	// The access id wins over the signed URL.
	fmt.Println(product.StableImageURL())
	// Without a small image access id, the LCSC URL wins over a signed URL.
	fmt.Println(product.StableThumbnailURL())
	fmt.Println(jlcpcb.IsSignedURL(product.ProductBigImageAccessIdUrl))
	// Output:
	// https://jlcpcb.com/api/file/downloadByFileSystemAccessId/8552476004850417664
	// https://assets.lcsc.com/images/lcsc/96x96/C1525_front.jpg
	// true
}

func ExampleFileService_Open() {
	client := jlcpcb.NewClient()
	body, info, err := client.File.Open(context.Background(), "8552476004850417664")
	if errors.Is(err, jlcpcb.ErrNotFound) {
		fmt.Println("JLCPCB has no file with this access id")
		return
	}
	if err != nil {
		fmt.Println("download error:", err)
		return
	}
	defer body.Close()

	// The server sends a wrong Content-Type for images. Open finds the
	// type from the bytes.
	if !strings.HasPrefix(info.ContentType, "image/") {
		fmt.Println("not an image:", info.ContentType)
		return
	}
	// Copy the bytes to a file or to a store. Do not store the signed URLs.
	n, err := io.Copy(io.Discard, body)
	if err != nil {
		fmt.Println("read error:", err)
		return
	}
	fmt.Println(info.FileName, info.ContentType, n)
}
