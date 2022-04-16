package proxy

import (
	abcicli "github.com/232425wxy/BFT/abci/client"
	srlog "github.com/232425wxy/BFT/libs/log"
	sros "github.com/232425wxy/BFT/libs/os"
	"github.com/232425wxy/BFT/libs/service"
	"fmt"
)

const (
	connConsensus = "consensus"
	connMempool   = "mempool"
	connQuery     = "query"
)

// AppConns 由几个 appconn 组成，并管理其底层的abci客户端。
type AppConns struct {
	service.BaseService

	consensusConn *AppConnConsensus
	mempoolConn   *AppConnMempool
	queryConn     *AppConnQuery
	consensusConnClient abcicli.Client
	mempoolConnClient   abcicli.Client
	queryConnClient     abcicli.Client

	clientCreator *ClientCreator
}

// NewAppConns makes all necessary abci connections to the application.
func NewAppConns(clientCreator *ClientCreator) *AppConns {
	multiAppConn := &AppConns{
		clientCreator: clientCreator,
	}
	multiAppConn.BaseService = *service.NewBaseService(nil, "AppConns", multiAppConn)
	return multiAppConn
}

func (app *AppConns) Mempool() *AppConnMempool {
	return app.mempoolConn
}

func (app *AppConns) Consensus() *AppConnConsensus {
	return app.consensusConn
}

func (app *AppConns) Query() *AppConnQuery {
	return app.queryConn
}

func (app *AppConns) OnStart() error {
	c, err := app.abciClientFor(connQuery)
	if err != nil {
		return err
	}
	app.queryConnClient = c
	app.queryConn = NewAppConnQuery(c)

	c, err = app.abciClientFor(connMempool)
	if err != nil {
		app.stopAllClients()
		return err
	}
	app.mempoolConnClient = c
	app.mempoolConn = NewAppConnMempool(c)

	c, err = app.abciClientFor(connConsensus)
	if err != nil {
		app.stopAllClients()
		return err
	}
	app.consensusConnClient = c
	app.consensusConn = NewAppConnConsensus(c)

	go app.killSROnClientError()

	return nil
}

func (app *AppConns) OnStop() {
	app.stopAllClients()
}

func (app *AppConns) killSROnClientError() {
	killFn := func(conn string, err error, logger srlog.CRLogger) {
		logger.Errorw(
			fmt.Sprintf("%s connection terminated. Did the application crash? Please restart SRBFT", conn),
			"err", err)
		killErr := sros.Kill()
		if killErr != nil {
			logger.Errorw("Failed to kill this process - please do so manually", "err", killErr)
		}
	}

	select {
	case <-app.consensusConnClient.Quit():
		if err := app.consensusConnClient.Error(); err != nil {
			killFn(connConsensus, err, app.Logger)
		}
	case <-app.mempoolConnClient.Quit():
		if err := app.mempoolConnClient.Error(); err != nil {
			killFn(connMempool, err, app.Logger)
		}
	case <-app.queryConnClient.Quit():
		if err := app.queryConnClient.Error(); err != nil {
			killFn(connQuery, err, app.Logger)
		}
	}
}

func (app *AppConns) stopAllClients() {
	if app.consensusConnClient != nil {
		if err := app.consensusConnClient.Stop(); err != nil {
			app.Logger.Errorw("error while stopping consensus client", "error", err)
		}
	}
	if app.mempoolConnClient != nil {
		if err := app.mempoolConnClient.Stop(); err != nil {
			app.Logger.Errorw("error while stopping mempool client", "error", err)
		}
	}
	if app.queryConnClient != nil {
		if err := app.queryConnClient.Stop(); err != nil {
			app.Logger.Errorw("error while stopping query client", "error", err)
		}
	}
}

func (app *AppConns) abciClientFor(conn string) (abcicli.Client, error) {
	c, err := app.clientCreator.NewABCIClient()
	if err != nil {
		return nil, fmt.Errorf("error creating ABCI client (%s connection): %w", conn, err)
	}
	c.SetLogger(app.Logger.With("module", "abci-client", "connection", conn))
	if err := c.Start(); err != nil {
		return nil, fmt.Errorf("error starting ABCI client (%s connection): %w", conn, err)
	}
	return c, nil
}
