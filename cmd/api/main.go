package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/auth"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/banks"
	abank "github.com/Shyyw1e/vtb-hackathon-smartfin/internal/banks/abank"
	sbank "github.com/Shyyw1e/vtb-hackathon-smartfin/internal/banks/sbank"
	vbank "github.com/Shyyw1e/vtb-hackathon-smartfin/internal/banks/vbank"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/cache"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/config"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/internal/handlers"
	"github.com/Shyyw1e/vtb-hackathon-smartfin/pkg/logger"
)

type banksProvider struct{ m map[string]banks.Client }

func (bp banksProvider) ClientByCode(code string) (banks.Client, bool) {
	c, ok := bp.m[code]
	return c, ok
}

func main() {
	cfg := config.MustLoad(os.Getenv("CONFIG"))
	logger.InitLog("debug")
	logger.Log.Infof("app=%s env=%s", cfg.App.Name, cfg.App.Env)

	ctx := context.Background()
	redisStore, err := cache.New(ctx, cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB)
	if err != nil {
		logger.Log.Fatalf("redis: %v", err)
	}
	defer redisStore.Close()

	httpc := &http.Client{
		Timeout: time.Second * time.Duration(cfg.Limits.BankTimeoutSec),
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   5 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			ForceAttemptHTTP2:     true,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}

	tokProv:= auth.NewTokenManager(httpc, cfg.Banks, redisStore)

	reqV, err := banks.NewRequester(auth.BankV, cfg.Banks["vbank"].BaseURL, httpc, tokProv, cfg)
	if err != nil {
		logger.Log.Fatalf("requester vbank: %v", err)
	}
	reqA, err := banks.NewRequester(auth.BankA, cfg.Banks["abank"].BaseURL, httpc, tokProv, cfg)
	if err != nil {
		logger.Log.Fatalf("requester abank: %v", err)
	}
	reqS, err := banks.NewRequester(auth.BankS, cfg.Banks["sbank"].BaseURL, httpc, tokProv, cfg)
	if err != nil {
		logger.Log.Fatalf("requester sbank: %v", err)
	}

	clients := map[string]banks.Client{
		"vbank": vbank.New(reqV),
		"abank": abank.New(reqA),
		"sbank": sbank.New(reqS),
	}
	bp := banksProvider{m: clients}

	dataH := &handlers.DataHandlers{Banks: bp}

	r := chi.NewRouter()
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Recoverer)
	r.Use(chimw.Timeout(15 * time.Second))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	r.Get("/api/v1/accounts", dataH.Accounts)
	r.Get("/api/v1/transactions", dataH.Transactions)

	srv := &http.Server{
		Addr:         cfg.App.HTTP.Addr,
		Handler:      r,
		ReadTimeout:  cfg.App.HTTP.ReadTimeout,
		WriteTimeout: cfg.App.HTTP.WriteTimeout,
		IdleTimeout:  60 * time.Second,
		BaseContext: func(_ net.Listener) context.Context { return context.Background() },
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Log.Infof("http: listening on %s", cfg.App.HTTP.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Log.Infof("signal: %s", sig)
	case err := <-errCh:
		logger.Log.Errorf("server error: %v", err)
	}

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctxShutdown)
	logger.Log.Info("shutdown complete")
}
