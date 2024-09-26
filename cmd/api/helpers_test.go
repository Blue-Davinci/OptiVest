package main

import (
	"testing"
	"time"

	"github.com/Blue-Davinci/OptiVest/internal/data"
)

func Test_application_isProfileComplete(t *testing.T) {
	type args struct {
		user *data.User
	}
	tests := []struct {
		name string
		app  *application
		args args
		want bool
	}{
		{
			name: "Complete Profile",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: true,
		},
		{
			name: "Missing FirstName",
			app:  &application{},
			args: args{
				user: &data.User{
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing LastName",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing Email",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing PhoneNumber",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing DOB",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					Address:      "123 Main St",
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing Address",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					CountryCode:  "US",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing CountryCode",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:    "John",
					LastName:     "Doe",
					Email:        "john.doe@example.com",
					PhoneNumber:  "1234567890",
					DOB:          time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:      "123 Main St",
					CurrencyCode: "USD",
				},
			},
			want: false,
		},
		{
			name: "Missing CurrencyCode",
			app:  &application{},
			args: args{
				user: &data.User{
					FirstName:   "John",
					LastName:    "Doe",
					Email:       "john.doe@example.com",
					PhoneNumber: "1234567890",
					DOB:         time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC),
					Address:     "123 Main St",
					CountryCode: "US",
				},
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.app.isProfileComplete(tt.args.user); got != tt.want {
				t.Errorf("application.isProfileComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}
