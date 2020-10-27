package roadrunner_temporal

import (
	"encoding/json"
	"github.com/spiral/endure/errors"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

// TODO: OPTIMIZE
func FromPayload(dc converter.DataConverter, payloads *commonpb.Payloads, values *[]json.RawMessage) error {
	if payloads == nil {
		return nil

	}
	*values = make([]json.RawMessage, 0, len(payloads.Payloads))

	payload := RRPayload{}
	if err := dc.FromPayloads(payloads, &payload); err != nil {
		return errors.E(errors.Op("decodePayload"), err)
	}

	// this is double serialization, we should remove it
	for _, value := range payload.Data {
		data, err := json.Marshal(value)
		if err != nil {
			return errors.E(errors.Op("encodeResult"), err)
		}

		*values = append(*values, data)
	}

	return nil
}

// TODO: OPTIMIZE
func ToPayload(dc converter.DataConverter, values []json.RawMessage, result *commonpb.Payloads) error {
	if len(values) == 0 {
		return nil
	}

	*result = commonpb.Payloads{
		Payloads: make([]*commonpb.Payload, 0, len(values)),
	}

	for _, value := range values {
		var raw interface{}

		err := json.Unmarshal(value, &raw)
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
