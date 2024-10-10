package data

import (
	"reflect"
	"testing"

	"github.com/shopspring/decimal"
)

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
				{Sector: "Basic Materials", ChangesPercentage: "0.51711%"},
				{Sector: "Technology", ChangesPercentage: "1.00374%"},
				{Sector: "Utilities", ChangesPercentage: "-3.33885%"},
			},
			args:    args{sectorName: "Basic Materials"},
			want:    decimal.RequireFromString("0.51711"),
			wantErr: false,
		},
		{
			name: "Valid sector - Utilities",
			s: SectorAnalysisData{
				{Sector: "Basic Materials", ChangesPercentage: "0.51711%"},
				{Sector: "Technology", ChangesPercentage: "1.00374%"},
				{Sector: "Utilities", ChangesPercentage: "-3.33885%"},
			},
			args:    args{sectorName: "Utilities"},
			want:    decimal.RequireFromString("-3.33885"),
			wantErr: false,
		},
		{
			name: "Sector not found",
			s: SectorAnalysisData{
				{Sector: "Basic Materials", ChangesPercentage: "0.51711%"},
				{Sector: "Technology", ChangesPercentage: "1.00374%"},
			},
			args:    args{sectorName: "Healthcare"},
			want:    decimal.Zero,
			wantErr: true,
		},
		{
			name: "Invalid percentage format",
			s: SectorAnalysisData{
				{Sector: "Technology", ChangesPercentage: "invalid%"},
			},
			args:    args{sectorName: "Technology"},
			want:    decimal.Zero,
			wantErr: true,
		},
		{
			name: "Case insensitive sector match",
			s: SectorAnalysisData{
				{Sector: "Basic Materials", ChangesPercentage: "0.51711%"},
				{Sector: "Technology", ChangesPercentage: "1.00374%"},
				{Sector: "Utilities", ChangesPercentage: "-3.33885%"},
			},
			args:    args{sectorName: "basic materials"}, // lowercase
			want:    decimal.RequireFromString("0.51711"),
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
