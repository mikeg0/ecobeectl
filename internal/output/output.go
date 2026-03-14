package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatCSV   Format = "csv"
)

func ParseFormat(s string) (Format, error) {
	switch Format(strings.ToLower(strings.TrimSpace(s))) {
	case "", FormatTable:
		return FormatTable, nil
	case FormatJSON:
		return FormatJSON, nil
	case FormatCSV:
		return FormatCSV, nil
	default:
		return "", fmt.Errorf("unsupported output format %q", s)
	}
}

func FilterFields(headers []string, rows []map[string]any, fields []string) ([]string, []map[string]any) {
	if len(fields) == 0 {
		return headers, rows
	}
	filteredHeaders := make([]string, 0, len(fields))
	filteredRows := make([]map[string]any, 0, len(rows))
	for _, field := range fields {
		filteredHeaders = append(filteredHeaders, field)
	}
	for _, row := range rows {
		filtered := map[string]any{}
		for _, field := range fields {
			filtered[field] = row[field]
		}
		filteredRows = append(filteredRows, filtered)
	}
	return filteredHeaders, filteredRows
}

func Print(w io.Writer, format Format, headers []string, rows []map[string]any) error {
	switch format {
	case FormatTable:
		return printTable(w, headers, rows)
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	case FormatCSV:
		return printCSV(w, headers, rows)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func printTable(w io.Writer, headers []string, rows []map[string]any) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if len(headers) > 0 {
		if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
			return err
		}
	}
	for _, row := range rows {
		parts := make([]string, 0, len(headers))
		for _, header := range headers {
			parts = append(parts, stringify(row[header]))
		}
		if _, err := fmt.Fprintln(tw, strings.Join(parts, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func printCSV(w io.Writer, headers []string, rows []map[string]any) error {
	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return err
	}
	for _, row := range rows {
		record := make([]string, 0, len(headers))
		for _, header := range headers {
			record = append(record, stringify(row[header]))
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
			return string(b[1 : len(b)-1])
		}
		return string(b)
	}
}
