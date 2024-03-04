package generic

import (
	"context"
	"errors"
	"go.keploy.io/server/v2/pkg/core/proxy/integrations"
	"go.keploy.io/server/v2/pkg/core/proxy/util"
	"go.keploy.io/server/v2/pkg/models"
	"go.uber.org/zap"
	"net"
)

func init() {
	integrations.Register("generic", NewGeneric)
}

type Generic struct {
	logger *zap.Logger
}

func NewGeneric(logger *zap.Logger) integrations.Integrations {
	return &Generic{
		logger: logger,
	}
}

func (g *Generic) MatchType(ctx context.Context, buffer []byte) bool {
	// generic is checked explicitly in the proxy
	return false
}

func (g *Generic) RecordOutgoing(ctx context.Context, src net.Conn, dst net.Conn, mocks chan<- *models.Mock, opts models.OutgoingOptions) error {
	logger := g.logger.With(zap.Any("Client IP Address", src.RemoteAddr().String()), zap.Any("Client ConnectionID", util.GetNextID()), zap.Any("Destination ConnectionID", util.GetNextID()))

	reqBuf, err := util.ReadInitialBuf(ctx, logger, src)
	if err != nil {
		logger.Error("failed to read the initial generic message", zap.Error(err))
		return errors.New("failed to record the outgoing generic call")
	}

	err = encodeGeneric(ctx, logger, reqBuf, src, dst, mocks, opts)
	if err != nil {
		logger.Error("failed to encode the generic message into the yaml", zap.Error(err))
		return errors.New("failed to record the outgoing generic call")
	}
	return nil
}

func (g *Generic) MockOutgoing(ctx context.Context, src net.Conn, dstCfg *integrations.ConditionalDstCfg, mockDb integrations.MockMemDb, opts models.OutgoingOptions) error {
	logger := g.logger.With(zap.Any("Client IP Address", src.RemoteAddr().String()), zap.Any("Client ConnectionID", util.GetNextID()), zap.Any("Destination ConnectionID", util.GetNextID()))

	reqBuf, err := util.ReadInitialBuf(ctx, logger, src)
	if err != nil {
		logger.Error("failed to read the initial generic message", zap.Error(err))
		return errors.New("failed to mock the outgoing generic call")
	}

	err = decodeGeneric(ctx, logger, reqBuf, src, dstCfg, mockDb, opts)
	if err != nil {
		logger.Error("failed to decode the generic message", zap.Error(err))
		return errors.New("failed to mock the outgoing generic call")
	}
	return nil
}