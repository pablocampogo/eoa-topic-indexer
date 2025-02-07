package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/pablocampogo/eoa-topic-indexer/balancer"
	"github.com/pablocampogo/eoa-topic-indexer/fetcher"
	"github.com/pablocampogo/eoa-topic-indexer/orchestrator"
	"github.com/pablocampogo/eoa-topic-indexer/persistor"
	"github.com/pablocampogo/eoa-topic-indexer/router"
	"github.com/pablocampogo/eoa-topic-indexer/synchronizer"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	persistor, err := persistor.NewPersistor()
	defer persistor.Close()
	if err != nil {
		panic(err)
	}

	balancer, err := balancer.NewBalancer()
	if err != nil {
		panic(err)
	}

	synchronizer := synchronizer.NewSynchronizer(persistor)
	fetcher := fetcher.NewFetcher(balancer, synchronizer)
	err = orchestrator.StartOrchestrator(synchronizer, fetcher, balancer, persistor)
	if err != nil {
		panic(err)
	}

	rt := router.NewRouter(persistor, synchronizer)

	rt.RunServer(ctx)
}
