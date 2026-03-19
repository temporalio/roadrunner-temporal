package helpers

import (
	"context"

	commonpb "go.temporal.io/api/common/v1"
	"go.temporal.io/sdk/converter"
)

// TestDataConverterPlugin is a minimal Endure plugin that implements converter.PayloadConverter.
// It delegates all operations to a standard JSON converter but uses a custom encoding string,
// allowing integration tests to verify that the custom data converter collection and configuration works.
type TestDataConverterPlugin struct {
	delegate converter.PayloadConverter
}

func (p *TestDataConverterPlugin) Init(Configurer) error {
	p.delegate = converter.NewJSONPayloadConverter()
	return nil
}

func (p *TestDataConverterPlugin) Serve() chan error {
	return make(chan error, 1)
}

func (p *TestDataConverterPlugin) Stop(context.Context) error {
	return nil
}

func (p *TestDataConverterPlugin) Encoding() string {
	return "json/test-custom"
}

func (p *TestDataConverterPlugin) ToPayload(value any) (*commonpb.Payload, error) {
	payload, err := p.delegate.ToPayload(value)
	if err != nil {
		return nil, err
	}
	// re-tag with our custom encoding so Temporal stores it with our metadata
	payload.Metadata["encoding"] = []byte(p.Encoding())
	return payload, nil
}

func (p *TestDataConverterPlugin) FromPayload(payload *commonpb.Payload, valuePtr any) error {
	return p.delegate.FromPayload(payload, valuePtr)
}

func (p *TestDataConverterPlugin) ToString(payload *commonpb.Payload) string {
	return p.delegate.ToString(payload)
}
