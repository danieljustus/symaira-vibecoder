package config

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// wireContract is loaded from .github/wire-contract.json at test time.
type wireContract struct {
	SchemaVersion int                         `json:"schema_version"`
	Comment       string                      `json:"comment"`
	Types         map[string]wireTypeContract `json:"types"`
}

type wireTypeContract struct {
	Keys       []string `json:"keys"`
	StatusEnum []string `json:"status_enum"`
}

func TestWireContractGoJSONTags(t *testing.T) {
	// Load the canonical contract.
	contract := loadContract(t)

	// Register every Go type we want to check. Each entry maps the contract
	// type name to a zero-value instance of the Go struct.
	goTypes := map[string]interface{}{
		"Cycle":          Cycle{},
		"Phase":          Phase{},
		"Step":           Step{},
		"AutoSkip":       AutoSkip{},
		"RequiresReview": RequiresReview{},
		"Model":          Model{},
		"WorkspaceConfig": WorkspaceConfig{},
	}

	for typeName, goVal := range goTypes {
		tc, ok := contract.Types[typeName]
		if !ok {
			t.Fatalf("contract missing type %q", typeName)
		}
		got := extractJSONTags(goVal)
		expected := normalizeKeys(tc.Keys)

		// Check for missing keys (in contract but not in Go).
		for _, k := range expected {
			if !contains(got, k) {
				t.Errorf("%s: missing Go json tag %q (present in contract)", typeName, k)
			}
		}
		// Check for extra keys (in Go but not in contract — may indicate a
		// field that needs to be added to the contract and Swift).
		for _, k := range got {
			if !contains(expected, k) {
				t.Errorf("%s: extra Go json tag %q (not in contract — add to contract and Swift)", typeName, k)
			}
		}
	}
}

func TestWireContractStatusEnum(t *testing.T) {
	contract := loadContract(t)
	tc, ok := contract.Types["StepStatus"]
	if !ok {
		t.Fatal("contract missing StepStatus type")
	}

	// Check that every StepStatus constant is in the contract.
	for _, s := range allStatusValues() {
		expected := string(s)
		if !contains(tc.StatusEnum, expected) {
			t.Errorf("StepStatus %q is not in contract", expected)
		}
	}
	// Check that every contract value has a corresponding constant.
	for _, v := range tc.StatusEnum {
		found := false
		for _, s := range allStatusValues() {
			if string(s) == v {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("contract StepStatus %q has no matching Go constant", v)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func loadContract(t *testing.T) wireContract {
	t.Helper()
	// Walk up from the test file location to find .github/wire-contract.json.
	// At test time, cwd is the repo root for `go test ./...`.
	data, err := os.ReadFile(".github/wire-contract.json")
	if err != nil {
		// Try alternative paths.
		data, err = os.ReadFile("../../.github/wire-contract.json")
		if err != nil {
			t.Fatalf("load contract: %v", err)
		}
	}
	var c wireContract
	if err := json.Unmarshal(data, &c); err != nil {
		t.Fatalf("parse contract: %v", err)
	}
	if c.SchemaVersion != 1 {
		t.Fatalf("unexpected contract schema version %d", c.SchemaVersion)
	}
	return c
}

// extractJSONTags returns all json tag values for a struct's DIRECT fields
// (not recursing into nested structs).
func extractJSONTags(v interface{}) []string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil
	}
	var tags []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.Split(tag, ",")[0]
		if name != "" {
			tags = append(tags, name)
		}
	}
	return tags
}

func normalizeKeys(keys []string) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = strings.TrimSpace(k)
	}
	return out
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// allStatusValues returns every defined StepStatus constant.
func allStatusValues() []StepStatus {
	return []StepStatus{
		StatusPending,
		StatusInProgress,
		StatusDone,
		StatusSkipped,
		StatusFailed,
		StatusBlocked,
		StatusNeedsReview,
	}
}
