package data

import (
	"reflect"
	"testing"

	"github.com/shopspring/decimal"
)

// dec is a tiny convenience to keep the table fixtures readable. The string
// form mirrors the wire format coming back from FMP's /stable endpoint.
func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	return decimal.RequireFromString(s)
}

func TestSectorAnalysisData_GetSectorChange(t *testing.T) {
	type args struct {
		sectorName string
	}
	tests := []struct {
		name    string
		s       SectorAnalysisData
		args    args
		want    decimal.Decimal
		wantErr bool
	}{
		{
			name: "Valid sector - Basic Materials",
			s: SectorAnalysisData{
				{Sector: "Basic Materials", AverageChange: dec(t, "0.51711")},
				{Sector: "Technology", AverageChange: dec(t, "1.00374")},
				{Sector: "Utilities", AverageChange: dec(t, "-3.33885")},
			},
			args:    args{sectorName: "Basic Materials"},
			want:    dec(t, "0.51711"),
			wantErr: false,
		},
		{
			name: "Valid sector - Utilities",
			s: SectorAnalysisData{
				{Sector: "Basic Materials", AverageChange: dec(t, "0.51711")},
				{Sector: "Technology", AverageChange: dec(t, "1.00374")},
				{Sector: "Utilities", AverageChange: dec(t, "-3.33885")},
			},
			args:    args{sectorName: "Utilities"},
			want:    dec(t, "-3.33885"),
			wantErr: false,
		},
		{
			name: "Sector not found",
			s: SectorAnalysisData{
				{Sector: "Basic Materials", AverageChange: dec(t, "0.51711")},
				{Sector: "Technology", AverageChange: dec(t, "1.00374")},
			},
			args:    args{sectorName: "Healthcare"},
			want:    decimal.Zero,
			wantErr: true,
		},
		{
			name: "Case insensitive sector match",
			s: SectorAnalysisData{
				{Sector: "Basic Materials", AverageChange: dec(t, "0.51711")},
				{Sector: "Technology", AverageChange: dec(t, "1.00374")},
				{Sector: "Utilities", AverageChange: dec(t, "-3.33885")},
			},
			args:    args{sectorName: "basic materials"},
			want:    dec(t, "0.51711"),
			wantErr: false,
		},
		{
			name: "First-match wins across exchanges",
			s: SectorAnalysisData{
				{Sector: "Technology", Exchange: "NASDAQ", AverageChange: dec(t, "1.0")},
				{Sector: "Technology", Exchange: "NYSE", AverageChange: dec(t, "0.5")},
			},
			args:    args{sectorName: "Technology"},
			want:    dec(t, "1.0"),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.s.GetSectorChange(tt.args.sectorName)
			if (err != nil) != tt.wantErr {
				t.Errorf("SectorAnalysisData.GetSectorChange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SectorAnalysisData.GetSectorChange() = %v, want %v", got, tt.want)
			}
		})
	}
}
