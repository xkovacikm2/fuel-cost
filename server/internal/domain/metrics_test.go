package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestConsumptionPer100Km(t *testing.T) {
	tests := []struct {
		name     string
		amount   string
		distance string
		want     string
	}{
		{name: "petrol", amount: "35.5", distance: "512.3", want: "6.93"},
		{name: "electricity", amount: "15", distance: "100", want: "15.00"},
		{name: "rounds half up", amount: "0.1005", distance: "10", want: "1.01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amount := decimal.RequireFromString(tt.amount)
			distance := decimal.RequireFromString(tt.distance)
			got := ConsumptionPer100Km(amount, distance)
			want := decimal.RequireFromString(tt.want)
			if !got.Equal(want) {
				t.Fatalf("ConsumptionPer100Km(%s, %s) = %s, want %s", amount, distance, got, want)
			}
		})
	}
}

func TestCostPer100Km(t *testing.T) {
	cost := decimal.RequireFromString("56.80")
	distance := decimal.RequireFromString("512.3")
	want := decimal.RequireFromString("11.09")

	if got := CostPer100Km(cost, distance); !got.Equal(want) {
		t.Fatalf("CostPer100Km(%s, %s) = %s, want %s", cost, distance, got, want)
	}
}
