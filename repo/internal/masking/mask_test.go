package masking

import "testing"

func TestMaskPhone(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"555-123-4567", "***-***-4567"},
		{"1234567890", "***-***-7890"},
		{"123", "****"},
		{"", "****"},
		{"1234", "***-***-1234"},
	}
	for _, tt := range tests {
		result := MaskPhone(tt.input)
		if result != tt.expected {
			t.Errorf("MaskPhone(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestMaskEmail(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"john@example.com", "j***@example.com"},
		{"a@b.com", "a***@b.com"},
		{"noemail", "****"},
		{"", "****"},
	}
	for _, tt := range tests {
		result := MaskEmail(tt.input)
		if result != tt.expected {
			t.Errorf("MaskEmail(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
