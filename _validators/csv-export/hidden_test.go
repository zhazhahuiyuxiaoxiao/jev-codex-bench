package csvexport_test

import (
	"encoding/csv"
	"strings"
	"testing"

	csvexport "example.com/csv-export"
)

func TestFormulaCells(t *testing.T) {
	cases := []string{"=SUM(1,2)", "+1+2", "-1+2", "@cmd", "  =1+2", "\t=1+2", "\r@cmd", "\n+1"}
	for _, input := range cases {
		got := csvexport.SafeCell(input)
		if !strings.HasPrefix(got, "'") || !strings.HasSuffix(got, input) {
			t.Errorf("unsafe cell %q became %q", input, got)
		}
	}
}

func TestSafeExportRoundTrip(t *testing.T) {
	encoded, err := csvexport.EncodeRow([]string{"=1+2", "ordinary"})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(encoded)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0][0] != "'=1+2" || rows[0][1] != "ordinary" {
		t.Fatalf("unexpected encoded row: %#v", rows)
	}
}
