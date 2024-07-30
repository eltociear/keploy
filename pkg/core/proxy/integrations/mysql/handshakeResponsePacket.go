//go:build linux

package mysql

import (
	"bytes"
	"encoding/binary"
	"errors"

	"go.keploy.io/server/v2/pkg/models"
)

const (
	// CLIENT_PLUGIN_AUTH indicates that the client supports plugin authentication.
	CLIENT_PLUGIN_AUTH = 0x080000
	// CLIENT_CONNECT_WITH_DB indicates that client is connecting to server with database name.
	CLIENT_CONNECT_WITH_DB = 0x08
	// CLIENT_CONNECT_ATTRS indicates that the client supports connection attributes.
	CLIENT_CONNECT_ATTRS = 0x0100000
	// CLIENT_ZSTD_COMPRESSION_ALGORITHM indicates support for Zstandard compression algorithm
	CLIENT_ZSTD_COMPRESSION_ALGORITHM = 0x010000
	// CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA indicates that the client supports plugin authentication with length encoded client data.
	CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA = 0x0200000
)

func decodeHandshakeResponse(data []byte) (*models.MySQLHandshakeResponse, error) {
	if len(data) < 32 {
		return nil, errors.New("handshake response packet too short")
	}

	packet := &models.MySQLHandshakeResponse{}

	packet.CapabilityFlags = binary.LittleEndian.Uint32(data[:4])
	data = data[4:]

	packet.MaxPacketSize = binary.LittleEndian.Uint32(data[:4])
	data = data[4:]

	packet.CharacterSet = data[0]
	data = data[1:]

	copy(packet.Reserved[:], data[:23])
	data = data[23:]

	idx := bytes.IndexByte(data, 0x00)
	if idx == -1 {
		return nil, errors.New("malformed handshake response packet: missing null terminator for Username")
	}
	packet.Username = string(data[:idx])
	data = data[idx+1:]

	if packet.CapabilityFlags&CLIENT_PLUGIN_AUTH_LENENC_CLIENT_DATA != 0 {
		length := int(data[0])
		data = data[1:]

		if length > 0 {
			if len(data) < length {
				return nil, errors.New("handshake response packet too short for auth data")
			}
			packet.AuthData = data[:length]
			data = data[length:]
		}
	} else {
		authLen := int(data[0])
		data = data[2:]
		packet.AuthData = data[:authLen]
	}

	if packet.CapabilityFlags&CLIENT_CONNECT_WITH_DB != 0 {
		idx = bytes.IndexByte(data, 0x00)
		if idx != -1 {
			packet.Database = string(data[:idx])
			data = data[idx+1:]
		}
	}

	if packet.CapabilityFlags&CLIENT_PLUGIN_AUTH != 0 {
		idx = bytes.IndexByte(data, 0x00)
		if idx == -1 {
			return nil, errors.New("malformed handshake response packet: missing null terminator for AuthPluginName")
		}
		packet.AuthPluginName = string(data[:idx])
		data = data[idx+1:]
	}

	if packet.CapabilityFlags&CLIENT_CONNECT_ATTRS != 0 {
		if len(data) < 4 {
			return nil, errors.New("handshake response packet too short for connection attributes")
		}

		totalLength, isNull, n := decodeLengthEncodedInteger(data)
		if isNull || n == 0 {
			return nil, errors.New("error decoding total length of connection attributes")
		}
		data = data[n:]

		attributesData := data[:totalLength]
		data = data[totalLength:]

		packet.ConnectAttributes = make(map[string]string)
		for len(attributesData) > 0 {
			keyLength, isNull, n := decodeLengthEncodedInteger(attributesData)
			if isNull {
				return nil, errors.New("malformed handshake response packet: null length encoded integer for connection attribute key")
			}
			attributesData = attributesData[n:]

			key := string(attributesData[:keyLength])
			attributesData = attributesData[keyLength:]

			valueLength, isNull, n := decodeLengthEncodedInteger(attributesData)
			if isNull {
				return nil, errors.New("malformed handshake response packet: null length encoded integer for connection attribute value")
			}
			attributesData = attributesData[n:]

			value := string(attributesData[:valueLength])
			attributesData = attributesData[valueLength:]

			packet.ConnectAttributes[key] = value
		}
	}
	if len(data) > 0 {
		if packet.CapabilityFlags&CLIENT_ZSTD_COMPRESSION_ALGORITHM != 0 {
			if len(data) < 1 {
				return nil, errors.New("handshake response packet too short for ZSTD compression level")
			}
			packet.ZstdCompressionLevel = data[0]
		}
	}

	return packet, nil
}
func decodeLengthEncodedInteger(b []byte) (length int, isNull bool, bytesRead int) {
	if len(b) == 0 {
		return 0, true, 0
	}

	switch b[0] {
	case 0xfb:
		return 0, true, 1
	case 0xfc:
		if len(b) < 3 {
			return 0, false, 0
		}
		return int(binary.LittleEndian.Uint16(b[1:3])), false, 3
	case 0xfd:
		if len(b) < 4 {
			return 0, false, 0
		}
		return int(b[1]) | int(b[2])<<8 | int(b[3])<<16, false, 4
	case 0xfe:
		if len(b) < 9 {
			return 0, false, 0
		}
		return int(binary.LittleEndian.Uint64(b[1:9])), false, 9
	default:
		return int(b[0]), false, 1
	}
}
