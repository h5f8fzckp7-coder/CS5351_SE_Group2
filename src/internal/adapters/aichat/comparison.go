package aichat

import (
	"context"
	"errors"
	"strings"

	"tangentry/internal/ai"
)

// CompareEvidence requests schema-constrained output through the existing
// completion adapter. Each explicitly rejected format may fall back only
// through another reservation on the same A invocation ledger.
func (c *Client) CompareEvidence(ctx context.Context, config ai.Config, input ai.ComparisonPrompt) (string, error) {
	if !ai.ValidEndpoint(config.BaseURL) || !ai.ValidModelName(config.Model) || input.ResponseFormat == nil {
		return "", ai.ErrInvalid
	}
	payload := map[string]any{
		"model": config.Model, "stream": false,
		"response_format": input.ResponseFormat,
		"messages":        []map[string]string{chatSystemMessage(config, input.Source), {"role": "user", "content": strings.TrimSpace(input.Question)}},
	}
	output, err := c.complete(ctx, config, payload)
	if errors.Is(err, ai.ErrJSONModeUnsupported) {
		payload["response_format"] = map[string]string{"type": "json_object"}
		output, err = c.complete(ctx, config, payload)
		if errors.Is(err, ai.ErrJSONModeUnsupported) {
			delete(payload, "response_format")
			return c.complete(ctx, config, payload)
		}
	}
	return output, err
}
