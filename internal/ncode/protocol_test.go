package ncode

import "testing"

func TestCheckTenantAndFilters(t *testing.T) {
	if err := CheckTenant([]byte(`{"tenantId":1}`)); err == nil {
		t.Fatal("tenant id must be rejected")
	}
	if err := CheckTenant([]byte(`{"filters":{"tenantId":"local"}}`)); err == nil {
		t.Fatal("filter tenant id must be rejected")
	}
	got, err := SanitizeFilters(map[string]any{"code": "600519", "sql": "1=1"}, []string{"code"})
	if err == nil || got != nil {
		t.Fatal("unknown filter must be rejected")
	}
	got, err = SanitizeFilters(map[string]any{"code": "600519"}, []string{"code"})
	if err != nil || got["code"] != "600519" {
		t.Fatalf("%v %+v", err, got)
	}
}

func TestSortAndPage(t *testing.T) {
	if err := CheckSort(&Sort{Field: "name", Method: "desc"}, []string{"createdAt"}); err == nil {
		t.Fatal("sort field")
	}
	if err := CheckSort(&Sort{Field: "createdAt", Method: "sideways"}, []string{"createdAt"}); err == nil {
		t.Fatal("sort method")
	}
	page, size := NormalizePage(0, 0)
	if page != 1 || size != 20 {
		t.Fatalf("default page %d %d", page, size)
	}
	if TotalPage(101, 20) != 6 {
		t.Fatal(TotalPage(101, 20))
	}
}

func TestAuthorize(t *testing.T) {
	if err := Authorize("Token abc", "", "stock:quote:view"); err == nil {
		t.Fatal("non bearer")
	}
	if err := Authorize("", "secret", "stock:quote:view"); err == nil {
		t.Fatal("missing bearer")
	}
	if err := Authorize("Bearer secret", "secret", "stock:quote:view"); err != nil {
		t.Fatal(err)
	}
}
