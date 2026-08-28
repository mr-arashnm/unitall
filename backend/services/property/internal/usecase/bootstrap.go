package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// HTTPBootstrap calls identity's internal bootstrap-manager endpoint via
// the gateway. It implements MembershipBootstrap.
type HTTPBootstrap struct {
	gatewayURL string // e.g. "http://gateway:8080"
	internalToken string
	httpClient *http.Client
}

// NewHTTPBootstrap returns a bootstrap client that POSTs to the gateway's
// /api/v1/internal/buildings/{id}/bootstrap-manager endpoint.
func NewHTTPBootstrap(gatewayURL, internalToken string) *HTTPBootstrap {
	return &HTTPBootstrap{
		gatewayURL:  gatewayURL,
		internalToken: internalToken,
		httpClient: &http.Client{},
	}
}

func (h *HTTPBootstrap) BootstrapManager(ctx context.Context, buildingID, userID string) error {
	body, _ := json.Marshal(map[string]string{"user_id": userID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		h.gatewayURL+"/api/v1/internal/buildings/"+buildingID+"/bootstrap-manager",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", h.internalToken)
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &bootstrapError{status: resp.StatusCode}
	}
	return nil
}

type bootstrapError struct {
	status int
}

func (e *bootstrapError) Error() string {
	return "bootstrap failed: " + http.StatusText(e.status)
}
