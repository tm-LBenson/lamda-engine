package lookup

import (
	"bytes"
	"compress/flate"
	"fmt"
	"io"
)

// The LUO1 envelope keeps its CRC and native-panel geometry. Larger public
// metadata may use a Z1-prefixed raw DEFLATE body; ordinary JSON stays valid.
func expandOverlayMetadata(b []byte) ([]byte, error) {
	if len(b) > 6134 {
		return nil, fmt.Errorf("overlay metadata exceeds packet")
	}
	if !bytes.HasPrefix(b, []byte("Z1")) {
		return b, nil
	}
	input := bytes.NewReader(b[2:])
	reader := flate.NewReader(input)
	data, err := io.ReadAll(io.LimitReader(reader, 65537))
	closeErr := reader.Close()
	if err != nil || closeErr != nil || len(data) > 65536 || input.Len() != 0 {
		return nil, fmt.Errorf("invalid compressed overlay metadata")
	}
	return data, nil
}
