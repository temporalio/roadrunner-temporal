package roadrunner_temporal

import (
	//"encoding/json"
	"errors"
	"fmt"

	json "github.com/json-iterator/go"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

//type (
//	// DataConverter is used by the framework to serialize/deserialize input and output of activity/workflow
//	// that need to be sent over the wire.
//	// To encode/decode workflow arguments, set DataConverter in client, through client.Options.
//	// To override DataConverter for specific activity or child workflow use workflow.WithDataConverter to create new Context,
//	// and pass that context to ExecuteActivity/ExecuteChildWorkflow calls.
//	// Temporal support using different DataConverters for different activity/childWorkflow in same workflow.
//	DataConverter interface {
//		// ToPayload converts single value to payload.
//		ToPayload(value interface{}) (*commonpb.Payload, error)
//		// FromPayload converts single value from payload.
//		FromPayload(payload *commonpb.Payload, valuePtr interface{}) error
//
//		// ToPayloads converts a list of values.
//		ToPayloads(value ...interface{}) (*commonpb.Payloads, error)
//		// FromPayloads converts to a list of values of different types.
//		// Useful for deserializing arguments of function invocations.
//		FromPayloads(payloads *commonpb.Payloads, valuePtrs ...interface{}) error
//
//		// ToString converts payload object into human readable string.
//		ToString(input *commonpb.Payload) string
//		// ToStrings converts payloads object into human readable strings.
//		ToStrings(input *commonpb.Payloads) []string
//	}
//)

type DataConverter struct {
}

type RRPayload struct {
	Data []interface{}
}

func NewRRDataConverter() converter.DataConverter {
	return &DataConverter{}
}

func (r *DataConverter) ToPayloads(values ...interface{}) (*commonpb.Payloads, error) {
	res := &commonpb.Payloads{}
	for i := 0; i < len(values); i++ {
		payload, err := r.ToPayload(values[i])
		if err != nil {
			return nil, fmt.Errorf("values[%d]: %w", i, err)
		}
		res.Payloads = append(res.Payloads, payload)
	}
	return res, nil
}

func (r *DataConverter) ToPayload(value interface{}) (*commonpb.Payload, error) {
	switch v := value.(type) {
	// special case
	case []byte:
		// according to the doc []byte encodes as a base64-encoded string
		// TODO bad operation converting bytes to string in such way
		b, err := json.Marshal(string(v))
		if err != nil {
			return nil, err
		}
		payload := &commonpb.Payload{
			Metadata: map[string][]byte{
				converter.MetadataEncoding: []byte(converter.MetadataEncodingBinary),
			},
			Data: b,
		}
		return payload, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		payload := &commonpb.Payload{
			Metadata: map[string][]byte{
				converter.MetadataEncoding: []byte(converter.MetadataEncodingJSON),
			},
			Data: b,
		}
		return payload, nil
	}
}

func (r *DataConverter) FromPayloads(payloads *commonpb.Payloads, valuePtrs ...interface{}) error {
	if payloads == nil {
		return nil
	}
	if len(valuePtrs) < 1 {
		return errors.New("valuePTRs len less than 0")
	}

	for i := 0; i < len(payloads.Payloads); i++ {
		err := r.FromPayload(payloads.Payloads[i], valuePtrs[0])
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *DataConverter) FromPayload(payload *commonpb.Payload, valuePtr interface{}) error {
	switch res := valuePtr.(type) {
	case *RRPayload:
		var data interface{}
		// TODO: BYPASS MARSHAL AND SEND IT AS IT IS
		err := json.Unmarshal(payload.GetData(), &data)
		if err != nil {
			return fmt.Errorf(
				"unable to decode argument: %T, with error: %v", valuePtr, err)
		}
		res.Data = append(res.Data, data)
	default:
		// todo: fallback to default converter
	}
	return nil
}

func (r *DataConverter) ToString(input *commonpb.Payload) string {
	panic("implement me")
}

func (r *DataConverter) ToStrings(input *commonpb.Payloads) []string {
	panic("implement me")
}
