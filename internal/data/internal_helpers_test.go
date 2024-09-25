package data

import (
	"encoding/base64"
	"reflect"
	"testing"
)

// Test_generateSecurityKey tests the generateSecurityKey function.
func Test_generateSecurityKey(t *testing.T) {
	tests := []struct {
		name      string
		keyLength int
		wantErr   bool
	}{
		{
			name:      "Valid AES-128 key (16 bytes)",
			keyLength: 16,
			wantErr:   false,
		},
		{
			name:      "Valid AES-192 key (24 bytes)",
			keyLength: 24,
			wantErr:   false,
		},
		{
			name:      "Valid AES-256 key (32 bytes)",
			keyLength: 32,
			wantErr:   false,
		},
		{
			name:      "Invalid key length (0 bytes)",
			keyLength: 0,
			wantErr:   true,
		},
		{
			name:      "Invalid key length (-1 bytes)",
			keyLength: -1,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateSecurityKey(tt.keyLength)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateSecurityKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && len(got) != tt.keyLength {
				t.Errorf("generateSecurityKey() = %v, want length %v", len(got), tt.keyLength)
			}
		})
	}
}

// Test_generateSecurityKeyUniqueness tests that two generated keys are not the same.
func Test_generateSecurityKeyUniqueness(t *testing.T) {
	keyLength := 32
	key1, err1 := generateSecurityKey(keyLength)
	key2, err2 := generateSecurityKey(keyLength)

	if err1 != nil || err2 != nil {
		t.Fatalf("Error generating keys: err1 = %v, err2 = %v", err1, err2)
	}

	if string(key1) == string(key2) {
		t.Errorf("generateSecurityKey() produced identical keys, which should not happen")
	}
}

func Test_encryptData(t *testing.T) {
	type args struct {
		data string
		key  []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "valid data and key",
			args: args{
				data: "Hello, World!",
				key:  []byte("1234567890123456"), // 16 bytes for AES-128
			},
			wantErr: false,
		},
		{
			name: "valid empty data",
			args: args{
				data: "",
				key:  []byte("1234567890123456"), // 16 bytes for AES-128
			},
			wantErr: false,
		},
		{
			name: "invalid key length (short)",
			args: args{
				data: "Hello",
				key:  []byte("shortkey"), // Less than 16 bytes
			},
			wantErr: true,
		},
		{
			name: "invalid key length (long)",
			args: args{
				data: "Hello",
				key:  []byte("this_key_is_too_long_for_aes"), // More than 32 bytes
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EncryptData(tt.args.data, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("encryptData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If there's no error, we check that the output is a valid base64 string.
			if !tt.wantErr {
				if _, err := base64.URLEncoding.DecodeString(got); err != nil {
					t.Errorf("encryptData() = %v, expected valid base64 string", got)
				}
			}
		})
	}
}

func TestDecryptData(t *testing.T) {
	type args struct {
		encryptedData string
		key           string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "Valid decryption",
			args: args{
				encryptedData: "BaxZEZiZZMXcfucFF8XzqtZz-fGllsGRiGUennVewgme",
				key:           "849322ee24911779da6917afa7faf3d5350fda64a992a0803346d4e74afa766c",
			},
			want:    "12345",
			wantErr: false,
		},
		{
			name: "Invalid decryption key",
			args: args{
				encryptedData: "TowTSSLcVHa_EPmmKFvtcwSy0J97uLX5CnElcQPXrSm6",
				key:           "wrongkey12345",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "Invalid encrypted data",
			args: args{
				encryptedData: "invaliddata",
				key:           "849322ee24911779da6917afa7faf3d5350fda64a992a0803346d4e74afa766c",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "Empty encrypted data",
			args: args{
				encryptedData: "",
				key:           "849322ee24911779da6917afa7faf3d5350fda64a992a0803346d4e74afa766c",
			},
			want:    "",
			wantErr: true,
		},
		{
			name: "Empty decryption key",
			args: args{
				encryptedData: "TowTSSLcVHa_EPmmKFvtcwSy0J97uLX5CnElcQPXrSm6",
				key:           "",
			},
			want:    "",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decodedKey, err := DecodeEncryptionKey(tt.args.key)
			if err != nil {
				if !tt.wantErr {
					t.Errorf("decodeEncryptionKey() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			got, err := DecryptData(tt.args.encryptedData, decodedKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecryptData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DecryptData() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeEncryptionKey(t *testing.T) {
	type args struct {
		encryptionKey string
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "Valid key",
			args: args{
				encryptionKey: "f86d9b52c465eb5d3a254d956eaea1f6763907246ae592f00faed2bbbd05d9b3",
			},
			want:    []byte{0xf8, 0x6d, 0x9b, 0x52, 0xc4, 0x65, 0xeb, 0x5d, 0x3a, 0x25, 0x4d, 0x95, 0x6e, 0xae, 0xa1, 0xf6, 0x76, 0x39, 0x07, 0x24, 0x6a, 0xe5, 0x92, 0xf0, 0x0f, 0xae, 0xd2, 0xbb, 0xbd, 0x05, 0xd9, 0xb3},
			wantErr: false,
		},
		{
			name: "Invalid key with non-hex characters",
			args: args{
				encryptionKey: "invalidkey12345",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Empty key",
			args: args{
				encryptionKey: "",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Short key",
			args: args{
				encryptionKey: "f86d9b52c465eb5d3a254d956eaea1f6",
			},
			want:    []byte{0xf8, 0x6d, 0x9b, 0x52, 0xc4, 0x65, 0xeb, 0x5d, 0x3a, 0x25, 0x4d, 0x95, 0x6e, 0xae, 0xa1, 0xf6},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecodeEncryptionKey(tt.args.encryptionKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeEncryptionKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DecodeEncryptionKey() = %v, want %v", got, tt.want)
			}
		})
	}
}
