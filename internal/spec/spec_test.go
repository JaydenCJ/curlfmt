// Spec-table tests: lookups, the --no- negation prefix, and structural
// invariants that guard the table against accidental edits.
package spec

import "testing"

func TestByShortResolvesCommonFlags(t *testing.T) {
	cases := map[byte]string{
		's': "silent", 'S': "show-error", 'L': "location", 'f': "fail",
		'X': "request", 'H': "header", 'd': "data", 'u': "user", 'o': "output",
	}
	for short, long := range cases {
		o, ok := ByShort(short)
		if !ok || o.Long != long {
			t.Fatalf("ByShort(%q) = %+v, %v; want %s", short, o, ok, long)
		}
	}
	if _, ok := ByShort('Q'); ok {
		t.Fatal("-Q must be unknown")
	}
}

func TestByLongResolvesValuesGroupsAndUnknowns(t *testing.T) {
	o, ok := ByLong("data-urlencode")
	if !ok || !o.TakesValue || o.Group != GroupData {
		t.Fatalf("data-urlencode = %+v, %v", o, ok)
	}
	o, ok = ByLong("compressed")
	if !ok || o.TakesValue {
		t.Fatalf("compressed = %+v, %v", o, ok)
	}
	if _, ok := ByLong("frobnicate"); ok {
		t.Fatal("frobnicate must be unknown")
	}
}

func TestNoPrefixNegation(t *testing.T) {
	// --no-silent negates a boolean and keeps its spelling.
	o, ok := ByLong("no-silent")
	if !ok || o.TakesValue || o.Long != "no-silent" {
		t.Fatalf("no-silent = %+v, %v", o, ok)
	}
	// --no-header is not a thing; negation only exists for booleans.
	if _, ok := ByLong("no-header"); ok {
		t.Fatal("no-header must be unknown")
	}
	// --no-buffer is a real long name (short -N), not a negation.
	o, ok = ByLong("no-buffer")
	if !ok || o.Short != 'N' {
		t.Fatalf("no-buffer = %+v, %v", o, ok)
	}
}

func TestTableStructuralInvariants(t *testing.T) {
	// No duplicate spellings, and SingleUse only on valued options —
	// booleans are idempotent in curl, so annotating one is a table bug.
	longs := map[string]bool{}
	shorts := map[byte]bool{}
	for _, o := range table {
		if longs[o.Long] {
			t.Fatalf("duplicate long option %q", o.Long)
		}
		longs[o.Long] = true
		if o.Short != 0 {
			if shorts[o.Short] {
				t.Fatalf("duplicate short option %q", o.Short)
			}
			shorts[o.Short] = true
		}
		if o.SingleUse && !o.TakesValue {
			t.Fatalf("%q is SingleUse but takes no value", o.Long)
		}
	}
}

func TestTableCoversARealisticSurface(t *testing.T) {
	if Count() < 90 {
		t.Fatalf("option table shrank to %d entries; the README documents 90+", Count())
	}
}
