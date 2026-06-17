// Package wire rewrites the text fields of each provider's native request/response
// JSON. There is no unified internal schema: an internal registry maps (provider, op)
// to a Provider implementation. Subpackages implement maintained providers. See
// docs/sdk/contract.md and docs/research/gateway-integration-survey.md.
//
// Status: implemented for Anthropic Messages and OpenAI Responses.
package wire
