package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gofor-little/env"
	_ "github.com/lib/pq"
	"greenlight.vysotsky.com/internal/data"
	"greenlight.vysotsky.com/internal/jsonlog"
)

const version = "1.0.0"

type config struct {
	port int
	env  string
	db   struct {
		dsn          string
		maxOpenConns int
		maxIdleConns int
		maxIdleTime  string
	}
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
}

type application struct {
	config config
	logger *jsonlog.Logger
	models data.Models
}

func main() {
	var conf config
	flag.IntVar(&conf.port, "port", 4000, "Api server port")
	flag.StringVar(&conf.env, "env", "development", "Environment (development|staging|pruduction)")

	if err := env.Load(".env"); err != nil {
		// panic(err)
	}

	default_dsn, _ := env.MustGet("DB_DSN")
	flag.StringVar(&conf.db.dsn, "db-dsn", default_dsn, "PostgreSQL DSN")
	flag.IntVar(&conf.db.maxOpenConns, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&conf.db.maxIdleConns, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&conf.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max idle time (10s|30m)")

	flag.Float64Var(&conf.limiter.rps, "limiter-rps", 2, "Rate limiter maximium requests per second")
	flag.IntVar(&conf.limiter.burst, "limiter-burst", 4, "Rate limiter maximium burst")
	flag.BoolVar(&conf.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")

	flag.Parse()

	if len(conf.db.dsn) == 0 {
		panic("database dsn was not provided neither in .env file nor as a -db-dsn flag")
	}

	fmt.Println("port:", conf.port)
	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)

	db, err := openDB(conf)
	if err != nil {
		logger.PrintFatal(err, nil)
	}

	defer db.Close()

	logger.PrintInfo("database connection established", nil)

	app := &application{
		config: conf,
		logger: logger,
		models: data.NewModels(db),
	}

	err = app.serve()

	if err != nil {
		logger.PrintFatal(err, nil)
	}
}

func openDB(cfg config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.db.maxOpenConns)
	db.SetMaxIdleConns(cfg.db.maxIdleConns)

	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}

	db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}