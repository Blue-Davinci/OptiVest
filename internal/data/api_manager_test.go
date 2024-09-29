package data

import (
	"reflect"
	"testing"

	"github.com/shopspring/decimal"
)

func TestConvertCurrency(t *testing.T) {
	type args struct {
		sourceAmount   decimal.Decimal
		conversionRate decimal.Decimal
	}
	tests := []struct {
		name string
		args args
		want ConvertedAmount
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ConvertCurrency(tt.args.sourceAmount, tt.args.conversionRate); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ConvertCurrency() = %v, want %v", got, tt.want)
			}
		})
	}
}
