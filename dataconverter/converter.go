package dataconverter

import (
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

// DataConverter wraps Temporal data converter to enable direct access to the payloads.
type DataConverter struct {
	converter.DataConverter
}

// NewDataConverter creates new data converter.
func NewDataConverter(fallback converter.DataConverter) converter.DataConverter {
	return &DataConverter{DataConverter: fallback}
}

// ToPayloads converts a list of values.
func (r *DataConverter) ToPayloads(values ...any) (*commonpb.Payloads, error) {
	for _, v := range values {
		if aggregated, ok := v.(*commonpb.Payloads); ok {
			// bypassing
			return aggregated, nil
		}
	}

	return r.DataConverter.ToPayloads(values...)
}

// FromPayloads converts to a list of values of different types.
// Useful for deserializing arguments of function invocations.
func (r *DataConverter) FromPayloads(payloads *commonpb.Payloads, valuePtrs ...any) error {
	if payloads == nil {
		return nil
	}

	if len(valuePtrs) == 1 {
		// input proxying
		if input, ok := valuePtrs[0].(**commonpb.Payloads); ok {
			*input = &commonpb.Payloads{}
			(*input).Payloads = payloads.Payloads
			return nil
		}
	}

	for i := 0; i < len(payloads.Payloads); i++ {
		err := r.FromPayload(payloads.Payloads[i], valuePtrs[i])
		if err != nil {
			return err
		}
	}

	return nil
}
