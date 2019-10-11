package ecspresso

import (
	"bytes"
	"encoding/json"

	"github.com/aws/aws-sdk-go/private/protocol/json/jsonutil"
)

func MarshalJSON(s interface{}) ([]byte, error) {
	var buf bytes.Buffer
	b, err := jsonutil.BuildJSON(s)
	if err != nil {
		return nil, err
	}
	json.Indent(&buf, b, "", "  ")
	buf.WriteString("\n")
	return buf.Bytes(), nil
}
