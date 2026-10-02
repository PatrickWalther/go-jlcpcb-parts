package jlcpcb

// PartsOrderQuote is the order state of a JLCPCB parts-shop order for one
// quantity. Product.PartsOrderQuote returns it.
type PartsOrderQuote struct {
	// PreOrder is true when JLCPCB fills the whole quantity as a pre-order.
	PreOrder bool
	// MinQty is the minimum order quantity for this state. It is 1 or more.
	MinQty int
	// Ladder is the price ladder for this state, sorted by StartNumber. It
	// is a copy of BuyComponentPrices for a pre-order, and a copy of
	// ComponentPrices for an order from stock. It is empty when the part
	// has no ladder for this state. The JLCPCB part page then shows
	// InitialPrice as the estimated unit price.
	Ladder []PriceBreak
}

// PartsOrderQuote returns the order state of a parts-shop order of qty
// parts. A qty below 1 counts as 1.
//
// PartsOrderQuote copies the parts-shop rule of the JLCPCB part pages:
//
//   - An order of qty <= CanPresaleNumber ships from stock. The ladder is
//     ComponentPrices.
//   - A larger order is a pre-order for the whole quantity. The ladder is
//     BuyComponentPrices.
//   - MinQty is max(MinPurchaseNum, PreMinPurchaseNum) when qty is larger
//     than CanPresaleNumber, when CanPresaleNumber is 0 or less, or when
//     CanPresaleNumber is less than MinPurchaseNum. Otherwise MinQty is
//     MinPurchaseNum.
//
// This is the rule for a parts order only. The limit for a PCBA order can
// differ: JLCPCB keeps part of its stock for PCBA orders, so a PCBA order
// probably draws on StockCount. This PCBA limit is inferred from UI text and
// is not verified.
//
// PartsOrderQuote does not check Buyable. JLCPCB does not sell a part when
// IsBuyComponent is "0".
func (p *Product) PartsOrderQuote(qty int) PartsOrderQuote {
	if qty < 1 {
		qty = 1
	}

	limit := p.CanPresaleNumber
	preOrder := qty > limit

	minQty := p.MinPurchaseNum
	if preOrder || limit <= 0 || limit < p.MinPurchaseNum {
		minQty = max(p.MinPurchaseNum, p.PreMinPurchaseNum)
	}
	if minQty < 1 {
		minQty = 1
	}

	ladder := p.SortedComponentPrices()
	if preOrder {
		ladder = p.SortedBuyComponentPrices()
	}

	return PartsOrderQuote{
		PreOrder: preOrder,
		MinQty:   minQty,
		Ladder:   ladder,
	}
}
