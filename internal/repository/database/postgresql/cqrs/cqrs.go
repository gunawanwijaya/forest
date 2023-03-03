package cqrs

import (
	"embed"
)

var (
	_ embed.FS

	//go:embed command_core_new_session.sql
	SQL_command_core_new_session string

	//go:embed command_core_new_user.sql
	SQL_command_core_new_user string

	//go:embed query_core_get_product.sql
	SQL_query_core_get_product string
)
