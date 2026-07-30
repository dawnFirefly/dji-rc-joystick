package output

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

// // // // // // // // // //

func jsonEncode(dst io.Writer, value any) error {
	encoder := json.NewEncoder(dst)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func escapeText(value string) string {
	quoted := strconv.Quote(value)
	return quoted[1 : len(quoted)-1]
}

// WriteJSON writes one compact JSON document followed by a newline.
func WriteJSON(dst io.Writer, value any) error {
	return jsonEncode(dst, value)
}

func formatFields(fields map[string]any) string {
	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for index, key := range keys {
		if index > 0 {
			builder.WriteByte(' ')
		}
		value := fields[key]
		if text, ok := value.(string); ok {
			value = escapeText(text)
		}
		_, _ = fmt.Fprintf(&builder, "%s=%v", escapeText(key), value)
	}
	return builder.String()
}
