package v1

import (
	"context"
	"encoding/binary"
	"errors"
	"go.keploy.io/server/v2/pkg/core/proxy/integrations"
	"go.keploy.io/server/v2/pkg/core/proxy/util"
	"net"

	"go.keploy.io/server/v2/pkg/models"

	"go.uber.org/zap"
)

func init() {
	integrations.Register("postgres_v1", NewPostgresV1)
}

type PostgresV1 struct {
	logger *zap.Logger
}

func NewPostgresV1(logger *zap.Logger) integrations.Integrations {
	return &PostgresV1{
		logger: logger,
	}
}

// MatchType determines if the outgoing network call is Postgres by comparing the
// message format with that of a Postgres text message.
func (p *PostgresV1) MatchType(ctx context.Context, reqBuf []byte) bool {
	const ProtocolVersion = 0x00030000 // Protocol version 3.0

	if len(reqBuf) < 8 {
		// Not enough data for a complete header
		return false
	}

	// The first four bytes are the message length, but we don't need to check those
	// The next four bytes are the protocol version
	version := binary.BigEndian.Uint32(reqBuf[4:8])
	if version == 80877103 {
		return true
	}
	return version == ProtocolVersion
}

func (p *PostgresV1) RecordOutgoing(ctx context.Context, src net.Conn, dst net.Conn, mocks chan<- *models.Mock, opts models.OutgoingOptions) error {
	logger := p.logger.With(zap.Any("Client IP Address", src.RemoteAddr().String()), zap.Any("Client ConnectionID", util.GetNextID()), zap.Any("Destination ConnectionID", util.GetNextID()))

	reqBuf, err := util.ReadInitialBuf(ctx, logger, src)
	if err != nil {
		logger.Error("failed to read the initial postgres message", zap.Error(err))
		return errors.New("failed to record the outgoing postgres call")
	}
	err = encodePostgres(ctx, logger, reqBuf, src, dst, mocks, opts)
	if err != nil {
		logger.Error("failed to encode the postgres message into the yaml", zap.Error(err))
		return errors.New("failed to record the outgoing postgres call")
	}
	return nil

}

func (p *PostgresV1) MockOutgoing(ctx context.Context, src net.Conn, dstCfg *integrations.ConditionalDstCfg, mockDb integrations.MockMemDb, opts models.OutgoingOptions) error {
	logger := p.logger.With(zap.Any("Client IP Address", src.RemoteAddr().String()), zap.Any("Client ConnectionID", util.GetNextID()), zap.Any("Destination ConnectionID", util.GetNextID()))

	reqBuf, err := util.ReadInitialBuf(ctx, logger, src)
	if err != nil {
		logger.Error("failed to read the initial postgres message", zap.Error(err))
		return errors.New("failed to mock the outgoing postgres call")
	}

	err = decodePostgres(ctx, logger, reqBuf, src, dstCfg, mockDb, opts)
	if err != nil {
		logger.Error("failed to decode the postgres message from the yaml", zap.Error(err))
		return errors.New("failed to mock the outgoing postgres call")
	}
	return nil
}