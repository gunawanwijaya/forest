package app1_http_get_homepage

import (
	"context"
	"net/http"

	"github.com/gunawanwijaya/forest/internal/repository/database/postgresql/postgresql_core"
	"github.com/gunawanwijaya/forest/sdk"
)

type Configuration struct{}

type Dependency struct {
	postgresql_core.PostgreSQLCore
}

type Flag struct{}

type instance struct {
	Configuration
	Dependency
	Flag
}

func New(ctx context.Context, c Configuration, d Dependency, f Flag) (App1HttpGetHomepage, error) {
	return &instance{}, nil
}

func Must(ctx context.Context, c Configuration, d Dependency, f Flag) App1HttpGetHomepage {
	x, err := New(ctx, c, d, f)
	sdk.PanicIf(err != nil, err)
	return x
}

type App1HttpGetHomepage interface {
	http.Handler
}

func (x *instance) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}
