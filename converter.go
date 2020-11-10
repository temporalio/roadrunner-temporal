package roadrunner_temporal

import (
	"errors"
	"fmt"

	jsoniter "github.com/json-iterator/go"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

type DataConverter struct {
	fallback converter.DataConverter
}

type RRPayload struct {
	Data []interface{} `json:"data"`
}

func NewDataConverter(fallback converter.DataConverter) converter.DataConverter {
	return &DataConverter{fallback: fallback}
}

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

func (r *DataConverter) ToPayload(value interface{}) (*commonpb.Payload, error) {
	return r.fallback.ToPayload(value)
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
		err := jsoniter.Unmarshal(payload.GetData(), &data)
		if err != nil {
			return fmt.Errorf(
				"unable to decode argument: %T, with error: %v", valuePtr, err)
		}
		res.Data = append(res.Data, data)
	default:
		return r.fallback.FromPayload(payload, valuePtr)
	}
	return nil
}

func (r *DataConverter) ToString(input *commonpb.Payload) string {
	return r.fallback.ToString(input)
}

func (r *DataConverter) ToStrings(input *commonpb.Payloads) []string {
	return r.fallback.ToStrings(input)
}
