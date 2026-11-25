package store

import "testing"

func TestValidRole(t *testing.T) {
	for _, r := range []string{"owner", "manajer", "buyer", "kasir"} {
		if !ValidRole(r) {
			t.Errorf("ValidRole(%q) = false", r)
		}
	}
	for _, r := range []string{"", "admin", "OWNER"} {
		if ValidRole(r) {
			t.Errorf("ValidRole(%q) = true", r)
		}
	}
}

func TestValidBranchForRole(t *testing.T) {
	if !ValidBranchForRole("owner", 0) {
		t.Error("owner tanpa cabang seharusnya boleh")
	}
	if ValidBranchForRole("kasir", 0) {
		t.Error("kasir tanpa cabang seharusnya ditolak")
	}
	if !ValidBranchForRole("manajer", 2) {
		t.Error("manajer dengan cabang seharusnya boleh")
	}
}

func TestValidPassword(t *testing.T) {
	if ValidPassword("1234567") {
		t.Error("password 7 karakter diterima")
	}
	if !ValidPassword("12345678") {
		t.Error("password 8 karakter ditolak")
	}
}
