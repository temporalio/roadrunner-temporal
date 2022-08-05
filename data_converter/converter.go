package data_converter //nolint:revive,stylecheck

import (
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

// DataConverter wraps Temporal data converter to enable direct access to the payloads.
type DataConverter struct {
	// dc is the RR data converter
	dc converter.DataConverter
}

// NewDataConverter creates new data converter.
func NewDataConverter(fallback converter.DataConverter) converter.DataConverter {
	return &DataConverter{dc: fallback}
}

// ToPayloads converts a list of values.
func (r *DataConverter) ToPayloads(values ...any) (*commonpb.Payloads, error) {
	for _, v := range values {
		if aggregated, ok := v.(*commonpb.Payloads); ok {
			// bypassing
			return aggregated, nil
		}
	}

	return r.dc.ToPayloads(values...)
}

// ToPayload converts single value to payload.
func (r *DataConverter) ToPayload(value any) (*commonpb.Payload, error) {
	return r.dc.ToPayload(value)
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

// FromPayload converts single value from payload.
func (r *DataConverter) FromPayload(payload *commonpb.Payload, valuePtr any) error {
	return r.dc.FromPayload(payload, valuePtr)
}

// ToString converts payload object into human readable string.
func (r *DataConverter) ToString(input *commonpb.Payload) string {
	return r.dc.ToString(input)
}

// ToStrings converts payloads object into human readable strings.
func (r *DataConverter) ToStrings(input *commonpb.Payloads) []string {
	return r.dc.ToStrings(input)
}
