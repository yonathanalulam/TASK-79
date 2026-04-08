package masking

// MaskPhone returns a masked phone number like ***-***-1234
func MaskPhone(phone string) string {
	if len(phone) < 4 {
		return "****"
	}
	last4 := phone[len(phone)-4:]
	return "***-***-" + last4
}

// MaskEmail returns a masked email like j***@example.com
func MaskEmail(email string) string {
	at := -1
	for i, c := range email {
		if c == '@' {
			at = i
			break
		}
	}
	if at <= 0 {
		return "****"
	}
	return string(email[0]) + "***" + email[at:]
}
