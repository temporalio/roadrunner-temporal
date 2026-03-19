// Package dataconverter wraps Temporal's data converter to bypass serialization when data
// is already in Payloads form, enabling efficient passthrough of protobuf payloads between Go and PHP workers.
package dataconverter
