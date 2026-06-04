package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// Timeout defines the maximum duration for GraphQL network operations.
var Timeout = getTimeoutFromEnv()

func getTimeoutFromEnv() time.Duration {
	if val := os.Getenv("HYPERRR_CLIENT_TIMEOUT"); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return 5 * time.Second
}

// GraphQLRequest defines the structure for outbound GraphQL operations.
type GraphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// GraphQLResponse wraps response payload format of GraphQL endpoints.
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// QueryGraphQL performs HTTP POST queries to retrieve and deserialize remote data.
func QueryGraphQL(serverURL string, query string, variables map[string]any, out any) error {
	reqBody, err := json.Marshal(GraphQLRequest{
		Query:     query,
		Variables: variables,
	})
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: Timeout}
	resp, err := client.Post(serverURL+"/query", "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned status code: %d", resp.StatusCode)
	}

	var gqlResp GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return err
	}

	if len(gqlResp.Errors) > 0 {
		return fmt.Errorf("graphql execution error: %s", gqlResp.Errors[0].Message)
	}

	return json.Unmarshal(gqlResp.Data, out)
}
