package bookmark

import "testing"

func TestValidateProxy(t *testing.T) {
	if err := ValidateProxy(Bookmark{ProxyMode: "inherit"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProxy(Bookmark{ProxyMode: "none"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProxy(Bookmark{
		ProxyMode: "custom", ProxyType: "http", ProxyHost: "p", ProxyPort: 3128,
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateProxy(Bookmark{ProxyMode: "custom", ProxyType: "http"}); err == nil {
		t.Fatal("expected error")
	}
}
