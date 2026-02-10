package jlcpcb_test

import (
	"context"
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
