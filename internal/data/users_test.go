package data

import (
	"testing"

	"github.com/Blue-Davinci/OptiVest/internal/validator"
)

func TestValidatePhoneNumber(t *testing.T) {
	type args struct {
		v            *validator.Validator
		phone_number string
		region       string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Valid US phone number with country code",
			args: args{
				v:            validator.New(),
				phone_number: "+14155552671",
				region:       "US",
			},
			wantErr: false,
		},
		{
			name: "Invalid US phone number",
			args: args{
				v:            validator.New(),
				phone_number: "+1415555267",
				region:       "US",
			},
			wantErr: true,
		},
		{
			name: "Valid Kenyan phone number with country code",
			args: args{
				v:            validator.New(),
				phone_number: "+254712223456",
				region:       "KE",
			},
			wantErr: false,
		},
		{
			name: "Invalid Kenyan phone number",
			args: args{
				v:            validator.New(),
				phone_number: "+254123456789",
				region:       "KE",
			},
			wantErr: true,
		},
		{
			name: "Valid German phone number with country code",
			args: args{
				v:            validator.New(),
				phone_number: "+4915123456789",
				region:       "DE",
			},
			wantErr: false,
		},
		{
			name: "Invalid German phone number",
			args: args{
				v:            validator.New(),
				phone_number: "+491512345678",
				region:       "DE",
			},
			wantErr: true,
		},
		{
			name: "Invalid format phone number",
			args: args{
				v:            validator.New(),
				phone_number: "invalid_number",
				region:       "US",
			},
			wantErr: true,
		},
		{
			name: "Empty phone number",
			args: args{
				v:            validator.New(),
				phone_number: "",
				region:       "US",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ValidatePhoneNumber(tt.args.v, tt.args.phone_number, tt.args.region)
			if (len(tt.args.v.Errors) > 0) != tt.wantErr {
				t.Errorf("ValidatePhoneNumber() error = %v, wantErr %v", tt.args.v.Errors, tt.wantErr)
			}
		})
	}
}
