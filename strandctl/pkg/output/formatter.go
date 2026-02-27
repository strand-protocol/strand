package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Formatter defines the interface for output formatting.
type Formatter interface {
	Format(data any) string
}

// NewFormatter returns a Formatter for the given format string.
// Supported formats: "table" (default), "json", "yaml".
func NewFormatter(format string) Formatter {
	switch strings.ToLower(format) {
	case "json":
		return &JSONFormatter{}
	case "yaml":
		return &YAMLFormatter{}
	default:
		return &TableFormatter{}
	}
}

// TableFormatter formats data as aligned text tables using tabwriter.
type TableFormatter struct{}

func (f *TableFormatter) Format(data any) string {
	var buf bytes.Buffer
	w := tabwriter.NewWriter(&buf, 0, 4, 2, ' ', 0)

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Slice:
		if v.Len() == 0 {
			return "No resources found.\n"
		}
		elem := v.Index(0)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		if elem.Kind() == reflect.Struct {
			t := elem.Type()
			// Print header
			headers := make([]string, t.NumField())
			for i := 0; i < t.NumField(); i++ {
				headers[i] = strings.ToUpper(t.Field(i).Name)
			}
			fmt.Fprintln(w, strings.Join(headers, "\t"))

			// Print rows
			for i := 0; i < v.Len(); i++ {
				row := v.Index(i)
				if row.Kind() == reflect.Ptr {
					row = row.Elem()
				}
				vals := make([]string, row.NumField())
				for j := 0; j < row.NumField(); j++ {
					vals[j] = fmt.Sprintf("%v", row.Field(j).Interface())
				}
				fmt.Fprintln(w, strings.Join(vals, "\t"))
			}
		} else {
			// Slice of non-struct (e.g., []string)
			for i := 0; i < v.Len(); i++ {
				fmt.Fprintln(w, v.Index(i).Interface())
			}
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			fmt.Fprintf(w, "%s:\t%v\n", t.Field(i).Name, v.Field(i).Interface())
		}
	default:
		fmt.Fprintln(w, data)
	}

	w.Flush()
	return buf.String()
}

// JSONFormatter formats data as indented JSON.
type JSONFormatter struct{}

func (f *JSONFormatter) Format(data any) string {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Sprintf("error formatting JSON: %v\n", err)
	}
	return string(b) + "\n"
}

// YAMLFormatter formats data as YAML.
type YAMLFormatter struct{}

func (f *YAMLFormatter) Format(data any) string {
	b, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Sprintf("error formatting YAML: %v\n", err)
	}
	return string(b)
}
