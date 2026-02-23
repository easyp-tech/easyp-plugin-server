package sdk

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/pluginpb"

	generator "github.com/easyp-tech/service/api/generator/v1"
)

// Client is a client for the EasyP API Service.
type Client struct {
	conn      *grpc.ClientConn
	genClient generator.ServiceAPIClient
	cfg       *config
	health    *healthMonitor // nil if health check is disabled
}

// NewClient creates a new Client connected to the specified address.
func NewClient(addr string, opts ...Option) (*Client, error) {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt.apply(cfg)
	}

	// Build interceptor chain: retry first, then user interceptors.
	interceptors := make([]grpc.UnaryClientInterceptor, 0, 1+len(cfg.unaryInterceptors))
	interceptors = append(interceptors, retryUnaryInterceptor(cfg.maxRetries, cfg.retryBaseDelay, cfg.retryMaxDelay))
	interceptors = append(interceptors, cfg.unaryInterceptors...)

	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(cfg.transportCreds),
		grpc.WithChainUnaryInterceptor(interceptors...),
	}

	if cfg.keepaliveParams != nil {
		dialOpts = append(dialOpts, grpc.WithKeepaliveParams(*cfg.keepaliveParams))
	}

	conn, err := grpc.NewClient(addr, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("grpc.NewClient %s: %w", addr, err)
	}

	c := &Client{
		conn:      conn,
		genClient: generator.NewServiceAPIClient(conn),
		cfg:       cfg,
	}

	if cfg.enableHealthCheck {
		c.health = &healthMonitor{
			conn:     conn,
			interval: cfg.healthCheckInterval,
			stopCh:   make(chan struct{}),
		}
		go c.health.start()
	}

	return c, nil
}

// Close closes the underlying gRPC connection.
// If a health monitor is running, it is stopped first.
func (c *Client) Close() error {
	if c.health != nil {
		c.health.stop()
	}
	return c.conn.Close()
}

// withTimeout returns a context with the earlier of the user's existing deadline
// or now+defaultTimeout. If the user has no deadline, defaultTimeout is used.
func (c *Client) withTimeout(ctx context.Context, defaultTimeout time.Duration) (context.Context, context.CancelFunc) {
	if userDeadline, ok := ctx.Deadline(); ok {
		deadline := time.Now().Add(defaultTimeout)
		if userDeadline.Before(deadline) {
			return ctx, func() {} // user deadline is earlier
		}
		return context.WithDeadline(ctx, deadline)
	}
	return context.WithTimeout(ctx, defaultTimeout)
}

// GenerateCode executes a plugin to generate code.
func (c *Client) GenerateCode(ctx context.Context, pluginName string, req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
	ctx, cancel := c.withTimeout(ctx, c.cfg.generateCodeTimeout)
	defer cancel()

	resp, err := c.genClient.GenerateCode(ctx, &generator.GenerateCodeRequest{
		PluginName:           pluginName,
		CodeGeneratorRequest: req,
	})
	if err != nil {
		return nil, fmt.Errorf("c.genClient.GenerateCode: %w", err)
	}

	return resp.CodeGeneratorResponse, nil
}

// ListPlugins retrieves a list of available plugins, optionally filtered.
func (c *Client) ListPlugins(ctx context.Context, filter ...PluginFilter) ([]*generator.PluginInfo, error) {
	ctx, cancel := c.withTimeout(ctx, c.cfg.listPluginsTimeout)
	defer cancel()

	resp, err := c.genClient.Plugins(ctx, &generator.PluginsRequest{})
	if err != nil {
		return nil, fmt.Errorf("c.genClient.Plugins: %w", err)
	}

	if len(filter) == 0 || filter[0].isEmpty() {
		return resp.Plugins, nil
	}

	return applyFilter(resp.Plugins, filter[0]), nil
}
