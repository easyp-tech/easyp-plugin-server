package connectors_test

import "github.com/easyp-tech/service/internal/database/connectors"

var (
	fullRawConfig = connectors.Raw{
		Query: "postgres://user:password@127.0.0.1:26257/defaultdb?application_name=application_name",
	}
)
