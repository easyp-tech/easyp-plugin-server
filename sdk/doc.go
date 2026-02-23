// Package sdk provides a client for the EasyP API Service.
//
// The SDK simplifies interaction with the EasyP API Service, allowing you to
// list available plugins and execute code generation requests remotely.
//
// Example usage:
//
//	c, err := sdk.NewClient("localhost:8080", sdk.WithInsecure())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer c.Close()
//
//	plugins, err := c.ListPlugins(context.Background())
//	if err != nil {
//	    log.Fatal(err)
//	}
package sdk
