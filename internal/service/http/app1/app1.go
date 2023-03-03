package app1

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gunawanwijaya/forest/internal/feature/app1_http_get_homepage"
	"github.com/gunawanwijaya/forest/internal/repository/database/postgresql/postgresql_core"
	"github.com/gunawanwijaya/forest/sdk"
)

var (
	sigs = []os.Signal{
		syscall.SIGABRT,
		syscall.SIGHUP,
		syscall.SIGKILL,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGSEGV,
	}
)

func Run() {
	ctx, cancel := signal.NotifyContext(context.Background(), sigs...)
	defer cancel()

	start := time.Now()
	s := new(Secret).Parse()
	c := new(Configuration).Parse()
	f := new(FeatureFlag).Parse()
	var err error

	// ===========================================================================
	// PREREQUISITE ==============================================================
	{
		logFile, err := os.CreateTemp(os.TempDir(), "")
		sdk.PanicIf(err != nil, err)
		sdk.PanicIf(logFile == nil, "logFile is nil")

		defer logFile.Close()

		log := sdk.OTel.NewLogger(ctx, logFile, sdk.OTel.NewConsoleWriter(os.Stdout))
		sdk.PanicIf(log == nil, "logger is nil")
		ctx = log.WithContext(ctx)
	}

	log := sdk.OTel.NewLogger(ctx).Z()
	sdk.PanicIf(log == nil, "log is nil")

	// ===========================================================================
	// REPOSITORY ================================================================
	postgresql_core := postgresql_core.Must(ctx,
		c.Repository.PostgreSqlCore,
		postgresql_core.Dependency{
			SQLConn: func() sdk.SQLConn {
				conns := make([]sdk.SQLConn, 0)
				for i, v := range s.Connection.Database.PostgreSQLCore {
					conn, err := new(sdk.SQL).OpenWithDSN(ctx, v)
					if i == 0 && (err != nil || conn == nil) {
						break
					}
					conns = append(conns, conn)
				}

				return new(sdk.SQL).NewRoundRobin(ctx, conns...)
			}(),
		},
	)

	// ===========================================================================
	// FEATURE ===================================================================
	app1_http_get_homepage := app1_http_get_homepage.Must(ctx,
		c.Feature.HttpGetHomepage,
		app1_http_get_homepage.Dependency{
			PostgreSQLCore: postgresql_core,
		},
		f.HttpGetHomepage,
	)

	// ===========================================================================
	// BUILD =====================================================================
	mux := new(sdk.Mux).
		Handle(http.MethodGet, "/", app1_http_get_homepage)

	srv := &http.Server{
		Addr:    ":10001",
		Handler: mux,
		// DisableGeneralOptionsHandler: false,
		TLSConfig:         nil,
		ReadTimeout:       time.Second,
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      time.Second,
		IdleTimeout:       time.Second,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
		// TLSNextProto: nil,
		// ConnState: nil,
		ErrorLog: sdk.OTel.NewLogger(ctx).S(),
		// BaseContext: nil,
		// ConnContext: nil,
	}
	srv.RegisterOnShutdown(func() {

	})

	log.Info().
		TimeDiff("duration", time.Now(), start).
		Str("addr", srv.Addr).
		Msg("[app1] is running")

	// ===========================================================================
	// LISTEN ====================================================================
	{
		start = time.Now()
		errChan := make(chan error, 1)

		go func() {
			switch srv.TLSConfig {
			case nil:
				err = srv.ListenAndServe()
			default:
				err = srv.ListenAndServeTLS("", "")
			}
			errChan <- err
		}()

		go func() {
			c := make(chan os.Signal, 1)
			signal.Notify(c, sigs...)

			select {
			case sig := <-c:
				print("\r")

				errChan <- fmt.Errorf("received signal: %s", sig)
			case <-ctx.Done():
				print("\r")

				errChan <- ctx.Err()
			}
		}()

		err = <-errChan

		log.Info().
			TimeDiff("duration", time.Now(), start).
			Msg("[app1] has been stopped")
		log.Err(err).Send()
	}

	// ===========================================================================
	// SHUTDOWN ==================================================================
	{
		start = time.Now()

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = srv.Shutdown(ctx)

		log.Info().
			TimeDiff("duration", time.Now(), start).
			Msg("[app1] has been shutdown")
		log.Err(err).Send()
	}

}

type Secret struct {
	Connection struct {
		Database struct {
			PostgreSQLCore []string
		}
	}
}

func (s *Secret) Parse() *Secret {
	return s
}

type Configuration struct {
	Feature struct {
		HttpGetHomepage app1_http_get_homepage.Configuration
	}
	Repository struct {
		PostgreSqlCore postgresql_core.Configuration
	}
}

func (c *Configuration) Parse() *Configuration {
	return c
}

type FeatureFlag struct {
	HttpGetHomepage app1_http_get_homepage.Flag
}

func (f *FeatureFlag) Parse() *FeatureFlag {
	return f
}
