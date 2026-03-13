package logging

import (
	"encoding/json"
	"fmt"
)

// JsonFormatter serializes LogRecord to JSON bytes.
type JsonFormatter struct{}

func NewJsonFormatter() *JsonFormatter {
	return &JsonFormatter{}
}

// Format returns JSON bytes and error if any occurred during serialization.
//
// Create a map of fields and serialize to JSON.
func (j *JsonFormatter) Format(record *LogRecord) ([]byte, error) {
	propertiesCount := len(record.Properties) / 2

	data := make(map[string]interface{}, 3+propertiesCount)
	data["timestamp"] = record.Timestamp
	data["level"] = record.Level.String()
	data["message"] = record.Message
	for i := 0; i < propertiesCount; i++ {
		key := fmt.Sprint(record.Properties[i*2])
		value := fmt.Sprint(record.Properties[i*2+1])
		data[key] = value
	}
	return json.Marshal(data)
}
