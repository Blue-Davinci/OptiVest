package data

import (
	"crypto/sha256"
	"testing"
	"time"
)

func TestRecoveryCodeDetail_Matches(t *testing.T) {
	// Test setup: Create valid recovery codes and their hash
	validRecoveryCodes := []string{"CODE1", "CODE2", "CODE3"}
	concatenatedCodes := "CODE1CODE2CODE3"
	hash := sha256.Sum256([]byte(concatenatedCodes))

	// Example `RecoveryCodeDetail` struct for testing
	details := &RecoveryCodeDetail{
		ID:     1,
		UserID: 123,
		RecoveryCodes: &RecoveryCodes{
			RecoveryCodes: validRecoveryCodes,
			CodeHash:      hash[:],
		},
		Used:      false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	tests := []struct {
		name    string
		p       *RecoveryCodeDetail
		args    string // plaintext password (concatenated recovery codes)
		want    bool
		wantErr bool
	}{
		{
			name:    "ValidRecoveryCodes",
			p:       details,
			args:    "CODE1CODE2CODE3", // Correct concatenation
			want:    true,
			wantErr: false,
		},
		{
			name:    "InvalidRecoveryCodes",
			p:       details,
			args:    "INVALIDCODE", // Incorrect concatenation
			want:    false,
			wantErr: false,
		},
		{
			name:    "EmptyRecoveryCodes",
			p:       details,
			args:    "", // Empty input
			want:    false,
			wantErr: false,
		},
		{
			name: "NilRecoveryCodesHash",
			p: &RecoveryCodeDetail{
				ID:     2,
				UserID: 456,
				RecoveryCodes: &RecoveryCodes{
					RecoveryCodes: validRecoveryCodes,
					CodeHash:      nil, // Simulating a missing hash
				},
				Used:      false,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			args:    "CODE1CODE2CODE3",
			want:    false,
			wantErr: false, // No error, but the match should fail
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.p.Matches(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("RecoveryCodeDetail.Matches() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("RecoveryCodeDetail.Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}
