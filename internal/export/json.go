package export

import "encoding/json"

// ExportJSON serializes ExportData to indented JSON bytes.
func ExportJSON(data *ExportData) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}
