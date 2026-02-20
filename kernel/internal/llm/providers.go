package llm

import (
	"context"
)

// Provider is the interface each LLM backend implements.
// It handles format translation (Jodo Format ↔ provider-specific format)
// and embedding generation. The proxy owns HTTP execution for chat.
type Provider interface {
	Name() string
	SupportsEmbed() bool

	// BuildRequest transforms a JodoRequest into a provider-specific HTTP request.
	BuildRequest(req *JodoRequest, model string) (*ProviderHTTPRequest, error)

	// ParseResponse transforms a provider-specific HTTP response into Jodo Format.
	ParseResponse(statusCode int, body []byte) (*ProviderHTTPResponse, error)

	// Embed generates an embedding vector. Providers handle their own HTTP for embed.
	Embed(ctx context.Context, model string, text string) ([]float32, int, error)
}
