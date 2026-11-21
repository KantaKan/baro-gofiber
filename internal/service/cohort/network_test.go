package cohort

import "testing"

func TestIPAllowed(t *testing.T) {
	cases := []struct {
		name    string
		allowed []string
		ip      string
		want    bool
	}{
		{"empty list allows any", nil, "1.2.3.4", true},
		{"inside cidr", []string{"10.0.0.0/8"}, "10.4.5.6", true},
		{"outside cidr", []string{"10.0.0.0/8"}, "11.0.0.1", false},
		{"single ip", []string{"203.0.113.7"}, "203.0.113.7", true},
		{"garbage entry ignored", []string{"garbage", "10.0.0.0/8"}, "10.0.0.1", true},
		{"ipv6 cidr", []string{"2001:db8::/32"}, "2001:db8::1", true},
		{"unparsable client ip", []string{"10.0.0.0/8"}, "nope", false},
	}
	for _, c := range cases {
		if got := IPAllowed(c.allowed, c.ip); got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
