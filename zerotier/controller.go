package zerotier

import (
	"fmt"

	"net/http"
	"strings"
)

// interface for authorizing zerotier members
type ZTController interface {
	Authorize(e *Endpoint) error
}

// official zerotier controller
type Controller struct {
	ZerotierToken string
}

func NewController(zerotierToken string) *Controller {
	return &Controller{ZerotierToken: zerotierToken}
}

var controllerEndpoint = "https://my.zerotier.com/api"

// authorize the endpoint (node) with the controller
func (c *Controller) AuthorizeMember(networkId string, memberAddress string, memberDescription string) error {

	loc := fmt.Sprintf("%s/network/%s/member/%s", controllerEndpoint, networkId, memberAddress)
	req, err := http.NewRequest("POST", loc, strings.NewReader(fmt.Sprintf(`{"config":{"authorized": true}, "annot": {"description": "%s"}}`, memberDescription)))
	if err != nil {
		return fmt.Errorf("Failed to create request: %s", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.ZerotierToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()
	if resp.StatusCode > 299 {
		return fmt.Errorf("Failed to update member details '%v': %s", req.Header, resp.Status)
	}

	return nil
}
