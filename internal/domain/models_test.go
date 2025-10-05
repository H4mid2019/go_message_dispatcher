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
		{
			name: "content exceeds 160 characters",
			message: Message{
				PhoneNumber: "+905551111111",
				Content:     "This is a very long message that exceeds the maximum allowed length of 160 characters. It contains way too much text and should definitely fail validation because SMS messages are limited.",
			},
			expectError: true,
		},
		{
			name: "content at exactly 160 characters",
			message: Message{
				PhoneNumber: "+905551111111",
				Content:     "This message is exactly one hundred sixty characters long to test the boundary condition for SMS length validation. We need to ensure it passes correctly!",
			},
			expectError: false,
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

func TestMessage_ValidateContent(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		maxLength   int
		expectError bool
	}{
		{"valid short message", "Hello world", 160, false},
		{"valid at max length", "This message is exactly one hundred sixty characters long to test the boundary condition for SMS length validation. We need to ensure it passes correctly!", 160, false},
		{"exceeds max length", "This is a very long message that exceeds the maximum allowed length of 160 characters. It contains way too much text and should definitely fail validation because SMS messages are limited.", 160, true},
		{"empty content", "", 160, true},
		{"valid with custom max", "Short", 10, false},
		{"exceeds custom max", "This is longer than 10", 10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := Message{Content: tt.content}
			err := msg.ValidateContent(tt.maxLength)
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
