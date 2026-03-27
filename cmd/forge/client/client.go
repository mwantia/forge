package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

const defaultAddr = "http://127.0.0.1:9280"

type ForgeClient struct {
	addr  string
	token string
	http  *http.Client
}

func resolveClient(addr, token string) *ForgeClient {
	if addr == "" {
		addr = os.Getenv("FORGE_HTTP_ADDR")
	}
	if addr == "" {
		addr = defaultAddr
	}
	if token == "" {
		token = os.Getenv("FORGE_HTTP_TOKEN")
	}
	return &ForgeClient{
		addr:  addr,
		token: token,
		http:  &http.Client{},
	}
}

func (c *ForgeClient) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.addr+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *ForgeClient) post(path string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.addr+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *ForgeClient) postRaw(path string, in any) (*http.Response, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, c.addr+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		return nil, parseErrorResponse(resp)
	}
	return resp, nil
}

func (c *ForgeClient) delete(path string) error {
	req, err := http.NewRequest(http.MethodDelete, c.addr+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *ForgeClient) do(req *http.Request, out any) error {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return parseErrorResponse(resp)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

func parseErrorResponse(resp *http.Response) error {
	var errResp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("%s: %s", errResp.Error.Code, errResp.Error.Message)
	}
	return fmt.Errorf("unexpected status %d", resp.StatusCode)
}
