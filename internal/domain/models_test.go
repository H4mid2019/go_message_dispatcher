package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMessage_IsValid(t *testing.T) {
	tests := []struct {
		name        string
		message     Message
		expectError bool
	}{
		{
			name: "valid message with standard phone",
			message: Message{
				PhoneNumber: "+905551111111",
				Content:     "Test message",
			},
			expectError: false,
		},
		{
			name: "valid message with different country code",
			message: Message{
				PhoneNumber: "+359888886645",
				Content:     "Test message",
			},
			expectError: false,
		},
		{
			name: "empty phone number",
			message: Message{
				PhoneNumber: "",
				Content:     "Test message",
			},
			expectError: true,
		},
		{
			name: "phone number too short",
			message: Message{
				PhoneNumber: "+12345",
				Content:     "Test message",
			},
			expectError: true,
		},
		{
			name: "phone number too long",
			message: Message{
				PhoneNumber: "+123456789012345678901",
				Content:     "Test message",
			},
			expectError: true,
		},
		{
			name: "empty content",
			message: Message{
				PhoneNumber: "+905551111111",
				Content:     "",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.IsValid()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMessage_ValidatePhoneNumber(t *testing.T) {
	tests := []struct {
		name        string
		phoneNumber string
		expected    bool
	}{
		{"valid turkish number", "+905551111111", true},
		{"valid bulgarian number", "+359888886645", true},
		{"valid standard number", "+12345678901", true},
		{"empty phone", "", false},
		{"too short", "+123", false},
		{"too long", "+123456789012345678901", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{PhoneNumber: tt.phoneNumber}
			result := msg.ValidatePhoneNumber()
			assert.Equal(t, tt.expected, result)
		})
	}
}
