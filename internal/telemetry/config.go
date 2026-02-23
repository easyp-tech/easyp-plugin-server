package telemetry

// Config contains telemetry settings.
type Config struct {
	OTLPEndpoint      string // OTEL_EXPORTER_OTLP_ENDPOINT, default "localhost:4317"
	ServiceName       string // OTEL_SERVICE_NAME, default "easyp-api-service"
	PyroscopeEndpoint string // PYROSCOPE_ENDPOINT, default "http://localhost:4040"
}
