package domain

import "github.com/shopspring/decimal"

var hundred = decimal.NewFromInt(100)

func ConsumptionPer100Km(amount, distance decimal.Decimal) decimal.Decimal {
	return amount.Div(distance).Mul(hundred).Round(2)
}

func CostPer100Km(cost, distance decimal.Decimal) decimal.Decimal {
	return cost.Div(distance).Mul(hundred).Round(2)
}