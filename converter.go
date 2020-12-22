package roadrunner_temporal

import (
	"fmt"
	"github.com/spiral/errors"

	jsoniter "github.com/json-iterator/go"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

type (
	DataConverter struct {
		fallback converter.DataConverter
	}

	RRPayload struct {
		Data []interface{} `json:"data"`
	}
)

func NewDataConverter(fallback converter.DataConverter) converter.DataConverter {
	return &DataConverter{fallback: fallback}
}

// ToPayloads converts a list of values.
func (r *DataConverter) ToPayloads(values ...interface{}) (*commonpb.Payloads, error) {
	res := &commonpb.Payloads{}
	for i := 0; i < len(values); i++ {
		if rrtP, ok := values[i].(RRPayload); ok {
			for j := 0; j < len(rrtP.Data); j++ {
				payload, err := r.ToPayload(rrtP.Data[j])
				if err != nil {
					return nil, fmt.Errorf("values[%d]: %w", i, err)
				}
				res.Payloads = append(res.Payloads, payload)
			}
			continue
		}

		payload, err := r.ToPayload(values[i])
		if err != nil {
			return nil, fmt.Errorf("values[%d]: %w", i, err)
		}
		res.Payloads = append(res.Payloads, payload)
	}

	return res, nil
}

// ToPayload converts single value to payload.
func (r *DataConverter) ToPayload(value interface{}) (*commonpb.Payload, error) {
	return r.fallback.ToPayload(value)
}

// FromPayloads converts to a list of values of different types.
// Useful for deserializing arguments of function invocations.
func (r *DataConverter) FromPayloads(payloads *commonpb.Payloads, valuePtrs ...interface{}) error {
	if payloads == nil {
		return nil
	}

	if len(valuePtrs) < 1 {
		return errors.E("valuePTRs len less than 0")
	}

	for i := 0; i < len(payloads.Payloads); i++ {
		err := r.FromPayload(payloads.Payloads[i], valuePtrs[0])
		if err != nil {
			return err
		}
	}

	return nil
}

// FromPayload converts single value from payload.
func (r *DataConverter) FromPayload(payload *commonpb.Payload, valuePtr interface{}) error {
	switch res := valuePtr.(type) {
	case *RRPayload:
		var data interface{}

		if len(payload.Data) == 0 {
			res.Data = append(res.Data, nil)
			return nil
		}

		// TODO: BYPASS MARSHAL AND SEND IT AS IT IS
		err := jsoniter.Unmarshal(payload.GetData(), &data)
		if err != nil {
			return errors.E(fmt.Sprintf("unable to decode argument: %T, with error: %v", valuePtr, err))
		}
		res.Data = append(res.Data, data)
	default:
		return r.fallback.FromPayload(payload, valuePtr)
	}
	return nil
}

// ToString converts payload object into human readable string.
func (r *DataConverter) ToString(input *commonpb.Payload) string {
	return r.fallback.ToString(input)
}

// ToStrings converts payloads object into human readable strings.
func (r *DataConverter) ToStrings(input *commonpb.Payloads) []string {
	return r.fallback.ToStrings(input)
}

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
