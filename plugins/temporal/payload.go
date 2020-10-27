package temporal

import (
	"encoding/json"
	"github.com/spiral/endure/errors"
	rrt "github.com/temporalio/roadrunner-temporal"
	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

// TODO: this method require some optizations
func ParsePayload(dc converter.DataConverter, payloads *commonpb.Payloads, values *[]json.RawMessage) error {
	if payloads == nil {
		return nil

	}
	*values = make([]json.RawMessage, 0, len(payloads.Payloads))

	payload := rrt.RRPayload{}
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
