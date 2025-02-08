package router

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/pablocampogo/eoa-topic-indexer/environment"
	"github.com/pablocampogo/eoa-topic-indexer/persistor"
	"github.com/pablocampogo/eoa-topic-indexer/synchronizer"
	"github.com/pablocampogo/eoa-topic-indexer/types"
	"golang.org/x/sync/errgroup"
)

var (
	// port defines the HTTP server port, configurable via the "PORT" environment variable (default: 8080).
	port = environment.GetString("PORT", "8080")
)

// Router handles HTTP endpoints for the indexer, providing health checks, data retrieval, and block tracking.
type Router struct {
	router       *mux.Router
	persistor    *persistor.Persistor
	synchronizer *synchronizer.Synchronizer
}

// NewRouter initializes a new Router instance, setting up HTTP endpoints for health checks,
// paginated data retrieval, and tracking the latest indexed block.
func NewRouter(p *persistor.Persistor, s *synchronizer.Synchronizer) *Router {
	rt := &Router{
		persistor:    p,
		synchronizer: s,
		router:       mux.NewRouter(),
	}

	rt.router.HandleFunc("/health", rt.healthCheck).Methods(http.MethodGet)
	rt.router.HandleFunc("/data", rt.getPaginated).Methods(http.MethodGet)
	rt.router.HandleFunc("/last-block-indexed", rt.getLatestBlockIndexed).Methods(http.MethodGet)

	return rt
}

func (rt *Router) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("EOA Topic Indexer is up and running!"))
}

func (rt *Router) getPaginated(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	startIndex, _ := strconv.Atoi(query.Get("startIndex"))
	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit <= 0 {
		limit = 10
	}

	data, nextIndex, err := rt.persistor.GetPaginated(startIndex, limit)
	if err != nil {
		fmt.Println(err.Error())
		http.Error(w, "Failed to fetch data", http.StatusInternalServerError)
		return
	}

	var nextPageURL string
	if len(data) == limit { // if there is less there's no more data
		nextPageURL = fmt.Sprintf("%s?startIndex=%d&limit=%d", r.URL.Path, nextIndex, limit)
	}

	response := &types.Response{
		Data:     data,
		NextPage: nextPageURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (rt *Router) getLatestBlockIndexed(w http.ResponseWriter, r *http.Request) {
	currBlock := rt.synchronizer.GetCurrBlock()

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(strconv.Itoa(currBlock - 1)))
}

// RunServer starts the HTTP server and listens for incoming requests.
// It gracefully shuts down when the provided context is canceled.
func (rt *Router) RunServer(ctx context.Context) {
	httpServer := &http.Server{
		Addr:    ":" + port,
		Handler: rt.router,
	}

	fmt.Println("EOA Topic Indexer running in port: ", port)

	g, gCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return httpServer.ListenAndServe()
	})
	g.Go(func() error {
		<-gCtx.Done()
		fmt.Println("HTTP router context finished")
		if err := httpServer.Shutdown(context.Background()); err != nil {
			fmt.Println("Error closing http server: ", err)
		}

		return nil
	})

	if err := g.Wait(); err != nil {
		fmt.Println("exit reason: ", err.Error())
	}
}
