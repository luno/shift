package shift

import (
	"encoding/json"
	"strconv"

	"github.com/luno/jettison/errors"
)

// Header is a key for structured event metadata.
type Header string

const (
	// HeaderRecordVersion is the entity's version at the time of the event.
	HeaderRecordVersion Header = "record_version"
)

// Headers is a map of structured event metadata.
type Headers map[Header]string

func encodeHeaders(h Headers) ([]byte, error) {
	return json.Marshal(h)
}

// DecodeHeaders decodes structured event metadata from a reflex event's MetaData field.
func DecodeHeaders(b []byte) (Headers, error) {
	var h Headers
	if err := json.Unmarshal(b, &h); err != nil {
		return nil, errors.Wrap(err, "decode headers")
	}
	return h, nil
}

// DecodeVersion is a convenience function that extracts the record version
// from a reflex event's MetaData field.
func DecodeVersion(metadata []byte) (int64, error) {
	h, err := DecodeHeaders(metadata)
	if err != nil {
		return 0, err
	}
	v, ok := h[HeaderRecordVersion]
	if !ok {
		return 0, errors.New("record_version header not found")
	}
	version, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, errors.Wrap(err, "parse record_version")
	}
	return version, nil
}
