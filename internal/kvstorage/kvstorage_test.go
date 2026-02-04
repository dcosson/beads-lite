package kvstorage

import (
	"errors"
	"testing"
)

func TestValidateTableName_Empty(t *testing.T) {
	err := ValidateTableName("")
	if err == nil {
		t.Fatal("expected error for empty table name")
	}
}

func TestValidateTableName_Reserved(t *testing.T) {
	for _, name := range ReservedTableNames {
		err := ValidateTableName(name)
		if err == nil {
			t.Errorf("expected error for reserved name %q", name)
		}
		if !errors.Is(err, ErrReservedTable) {
			t.Errorf("ValidateTableName(%q) error = %v, want ErrReservedTable", name, err)
		}
	}
}

func TestValidateTableName_Valid(t *testing.T) {
	validNames := []string{"slots", "agents", "my-table", "merge-slot"}
	for _, name := range validNames {
		if err := ValidateTableName(name); err != nil {
			t.Errorf("ValidateTableName(%q) unexpected error: %v", name, err)
		}
	}
}
