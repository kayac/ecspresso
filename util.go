package ecspresso

import (
	"bytes"
	"encoding/json"

	"github.com/aws/aws-sdk-go/private/protocol/json/jsonutil"
)

func marshalJSON(s interface{}) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	b, err := jsonutil.BuildJSON(s)
	if err != nil {
		return nil, err
	}
	json.Indent(&buf, b, "", "  ")
	buf.WriteString("\n")
	return &buf, nil
}

func MarshalJSON(s interface{}) ([]byte, error) {
	b, err := marshalJSON(s)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), err
}

func MarshalJSONString(s interface{}) string {
	b, _ := marshalJSON(s)
	return b.String()
}
