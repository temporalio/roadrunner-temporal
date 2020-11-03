package roadrunner_temporal

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/spiral/errors"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

// TODO: OPTIMIZE
func FromPayloads(dc converter.DataConverter, payloads *commonpb.Payloads, values *[]jsoniter.RawMessage) error {
	if payloads == nil {
		return nil

	}
	*values = make([]jsoniter.RawMessage, 0, len(payloads.Payloads))

	payload := RRPayload{}
	if err := dc.FromPayloads(payloads, &payload); err != nil {
		return errors.E(errors.Op("decodePayload"), err)
	}

	// this is double serialization, we should remove it
	for _, value := range payload.Data {
		data, err := jsoniter.Marshal(value)
		if err != nil {
			return errors.E(errors.Op("encodeResult"), err)
		}

		*values = append(*values, data)
	}

	return nil
}

// TODO: OPTIMIZE
func ToPayloads(dc converter.DataConverter, values []jsoniter.RawMessage, result *commonpb.Payloads) error {
	if len(values) == 0 {
		return nil
	}

	*result = commonpb.Payloads{
		Payloads: make([]*commonpb.Payload, 0, len(values)),
	}

	for _, value := range values {
		var raw interface{}

		err := jsoniter.Unmarshal(value, &raw)
		if err != nil {
			return err
		}

		out, err := dc.ToPayload(raw)
		if err != nil {
			return err
		}

		result.Payloads = append(result.Payloads, out)
	}

	return nil
}
