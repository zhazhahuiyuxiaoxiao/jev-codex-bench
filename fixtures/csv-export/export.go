package csvexport

import (
	"bytes"
	"encoding/csv"
)

func SafeCell(value string) string {
	return value
}

func EncodeRow(values []string) (string, error) {
	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	row := make([]string, len(values))
	for i, value := range values {
		row[i] = SafeCell(value)
	}
	if err := writer.Write(row); err != nil {
		return "", err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return out.String(), nil
}
