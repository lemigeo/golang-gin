package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"golang-gin/internal/config"
	"golang-gin/internal/handler"
)

const shutdownTimeout = 10 * time.Second

// @title                     golang-gin API
// @version                   1.0
// @description               Customer sign-up API. Endpoint specs live in the
// @description               handler annotations, not in README.md.
// @host                      localhost:8080
// @BasePath                  /
// @accept                    json
// @produce                   json

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
	log.Println("stopped")
}

func run() error {
	addr, err := config.HTTPAddr()
	if err != nil {
		return err
	}
	dsn, err := config.DBDSN()
	if err != nil {
		return err
	}
	redisAddr, err := config.RedisAddr()
	if err != nil {
		return err
	}
	sessionSecret, err := config.SessionSecret()
	if err != nil {
		return err
	}
	sessionTTL, err := config.SessionTTL()
	if err != nil {
		return err
	}

	// Cancelled on SIGINT/SIGTERM, which is what ends the wait below.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	pingCtx, cancelPing := context.WithTimeout(ctx, 5*time.Second)
	defer cancelPing()
	if pingErr := db.PingContext(pingCtx); pingErr != nil {
		return pingErr
	}

	lc := net.ListenConfig{KeepAlive: 3 * time.Minute}
	ln, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: os.Getenv("REDIS_PASSWORD"),
	})
	defer func() { _ = rdb.Close() }()

	redisCtx, cancelRedis := context.WithTimeout(ctx, 5*time.Second)
	defer cancelRedis()
	if redisErr := rdb.Ping(redisCtx).Err(); redisErr != nil {
		return redisErr
	}

	engine := handler.NewEngine(db, handler.Config{
		KakaoBaseURL:  os.Getenv("KAKAO_API_BASE_URL"),
		NaverBaseURL:  os.Getenv("NAVER_API_BASE_URL"),
		Redis:         rdb,
		SessionSecret: sessionSecret,
		SessionTTL:    sessionTTL,
	})

	srv := &http.Server{
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		// ln.Addr() is what the socket actually bound to, so ":0" reports the
		// real port and no environment value reaches the log.
		log.Printf("listening on %s", ln.Addr())
		if sErr := srv.Serve(ln); sErr != nil && !errors.Is(sErr, http.ErrServerClosed) {
			serveErr <- sErr
			return
		}
		serveErr <- nil
	}()

	select {
	case serveDone := <-serveErr:
		return serveDone
	case <-ctx.Done():
		stop() // a second signal now kills the process instead of being swallowed
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	return srv.Shutdown(shutdownCtx)
}
