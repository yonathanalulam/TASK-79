package cart

import (
	"testing"
)

func TestErrItemNotInCart(t *testing.T) {
	if ErrItemNotInCart == nil {
		t.Fatal("ErrItemNotInCart should not be nil")
	}
	if ErrItemNotInCart.Error() != "item does not belong to this cart" {
		t.Errorf("unexpected error message: %s", ErrItemNotInCart.Error())
	}
}

func TestCartStatusValues(t *testing.T) {
	validStatuses := []string{"open", "submitted", "converted", "abandoned"}
	for _, s := range validStatuses {
		c := Cart{Status: s}
		if c.Status != s {
			t.Errorf("unexpected status: %s", c.Status)
		}
	}
}

func TestCartItemValidityStatuses(t *testing.T) {
	validStatuses := []string{"valid", "discontinued", "unpublished", "out_of_stock"}
	for _, s := range validStatuses {
		ci := CartItem{ValidityStatus: s}
		if ci.ValidityStatus != s {
			t.Errorf("unexpected validity status: %s", ci.ValidityStatus)
		}
	}
}

func TestMergeErrors(t *testing.T) {
	if ErrCartNotOpen == nil || ErrMergeSameCart == nil || ErrMergeDiffCustomer == nil {
		t.Fatal("merge error constants should not be nil")
	}
}
