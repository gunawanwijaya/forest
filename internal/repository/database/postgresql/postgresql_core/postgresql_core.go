package postgresql_core

import (
	"context"

	"github.com/gunawanwijaya/forest/internal/repository/database/postgresql/cqrs"
	"github.com/gunawanwijaya/forest/sdk"
)

type Configuration struct{}

type Dependency struct {
	sdk.SQLConn
}

type instance struct {
	Configuration
	Dependency
}

func New(ctx context.Context, c Configuration, d Dependency) (PostgreSQLCore, error) {
	return &instance{}, nil
}

func Must(ctx context.Context, c Configuration, d Dependency) PostgreSQLCore {
	x, err := New(ctx, c, d)
	sdk.PanicIf(err != nil, err)
	return x
}

type PostgreSQLCore interface {
	NewSession(ctx context.Context, req NewSessionRequest) (res NewSessionResponse, err error)
	NewUser(ctx context.Context, req NewUserRequest) (res NewUserResponse, err error)
	GetProduct(ctx context.Context, req GetProductRequest) (res GetProductResponse, err error)
}

type NewSessionRequest struct {
	//
}
type NewSessionResponse struct {
	//
}

func (x *instance) NewSession(ctx context.Context, req NewSessionRequest) (res NewSessionResponse, err error) {
	rowsAffected := 0
	lastInsertID := 0
	err = new(sdk.SQL).
		BoxExec(x.SQLConn.ExecContext(ctx, cqrs.SQL_command_core_new_session)).
		Scan(&rowsAffected, &lastInsertID)
	return res, err
}

type NewUserRequest struct {
	//
}
type NewUserResponse struct {
	//
}

func (x *instance) NewUser(ctx context.Context, req NewUserRequest) (res NewUserResponse, err error) {
	rowsAffected := 0
	lastInsertID := 0
	err = new(sdk.SQL).
		BoxExec(x.SQLConn.ExecContext(ctx, cqrs.SQL_command_core_new_user)).
		Scan(&rowsAffected, &lastInsertID)
	return res, err
}

type GetProductRequest struct {
	//
}
type GetProductResponse struct {
	List []GetProductResponse

	ID []byte
}

func (x *instance) GetProduct(ctx context.Context, req GetProductRequest) (res GetProductResponse, err error) {
	err = new(sdk.SQL).
		BoxQuery(x.SQLConn.QueryContext(ctx, cqrs.SQL_query_core_get_product)).
		Scan(func(i int) sdk.List {
			res.List = append(res.List, GetProductResponse{})
			return sdk.List{
				&res.List[i].ID,
			}
		})
	return res, err
}
