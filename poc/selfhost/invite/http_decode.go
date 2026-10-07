package invite

import (
	"bytes"
	"encoding/json"
	"io"

	"remoteapp.local/selfhost-poc/privatepipe"
)

// Decode a single bounded HTTP JSON document, preserving support for pretty
// JSON while rejecting duplicate/case-alias keys at every nested object.
func decodeHTTP(r io.Reader, value any) error {
	b, e := io.ReadAll(r)
	if e != nil {
		return e
	}
	var compact bytes.Buffer
	if e = json.Compact(&compact, b); e != nil {
		return e
	}
	p := privatepipe.NewReader(bytes.NewReader(append(compact.Bytes(), '\n')))
	if e = p.Read(value); e != nil {
		return e
	}
	var extra struct{}
	if p.Read(&extra) != io.EOF {
		return privatepipe.ErrProtocol
	}
	return nil
}

// DecodeDeviceRequest is bounded independently of the HTTP caller. No store
// operation occurs here; the authority must still verify the device signature.
func DecodeDeviceRequest(r io.Reader) (Envelope, error) {
	var e Envelope
	b, err := io.ReadAll(io.LimitReader(r, 4097))
	if err != nil || len(b) > 4096 {
		return e, ErrDenied
	}
	if decodeHTTP(bytes.NewReader(b), &e) != nil {
		return Envelope{}, ErrDenied
	}
	return e, nil
}

// DecodeNetworkRequest does not authorize; only the Store may do so.
func DecodeNetworkRequest(r io.Reader) (NetworkRequest, error) {
	var e NetworkRequest
	b, err := io.ReadAll(io.LimitReader(r, 8193))
	if err != nil || len(b) > 8192 || decodeHTTP(bytes.NewReader(b), &e) != nil {
		return NetworkRequest{}, ErrDenied
	}
	return e, nil
}
