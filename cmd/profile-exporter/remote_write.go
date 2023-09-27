// Copyright 2023 Polar Signals Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	prometheus "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	config_util "github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/sigv4"
	"github.com/prometheus/prometheus/storage/remote/azuread"
)

const (
	maxErrMsgLen = 1024
)

// ClientConfig configures a client.
type ClientConfig struct {
	URL              *config_util.URL
	Timeout          model.Duration
	HTTPClientConfig config_util.HTTPClientConfig
	SigV4Config      *sigv4.SigV4Config
	AzureADConfig    *azuread.AzureADConfig
	Headers          map[string]string
}

type Client struct {
	client    *http.Client
	urlString string
	timeout   time.Duration

	buf  []byte
	pBuf *proto.Buffer
}

func NewClient(conf *ClientConfig) (*Client, error) {
	httpClient, err := config_util.NewClientFromConfig(conf.HTTPClientConfig, "profile_exporter")
	if err != nil {
		return nil, err
	}
	t := httpClient.Transport

	if conf.SigV4Config != nil {
		t, err = sigv4.NewSigV4RoundTripper(conf.SigV4Config, httpClient.Transport)
		if err != nil {
			return nil, err
		}
	}

	if conf.AzureADConfig != nil {
		t, err = azuread.NewAzureADRoundTripper(conf.AzureADConfig, httpClient.Transport)
		if err != nil {
			return nil, err
		}
	}

	if len(conf.Headers) > 0 {
		t = newInjectHeadersRoundTripper(conf.Headers, t)
	}
	httpClient.Transport = t

	return &Client{
		urlString: conf.URL.String(),
		client:    httpClient,
		timeout:   time.Duration(conf.Timeout),

		pBuf: proto.NewBuffer(nil),
		buf:  make([]byte, 1024),
	}, nil
}

func newInjectHeadersRoundTripper(h map[string]string, underlyingRT http.RoundTripper) *injectHeadersRoundTripper {
	return &injectHeadersRoundTripper{headers: h, RoundTripper: underlyingRT}
}

type injectHeadersRoundTripper struct {
	headers map[string]string
	http.RoundTripper
}

func (t *injectHeadersRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}
	return t.RoundTripper.RoundTrip(req)
}

func (c *Client) Send(ctx context.Context, wreq *prometheus.WriteRequest) error {
	c.pBuf.Reset()
	err := c.pBuf.Marshal(wreq)
	if err != nil {
		return err
	}

	// snappy uses len() to see if it needs to allocate a new slice. Make the
	// buffer as long as possible.
	if c.buf != nil {
		c.buf = c.buf[0:cap(c.buf)]
	}
	c.buf = snappy.Encode(c.buf, c.pBuf.Bytes())

	return c.sendReq(ctx, c.buf)
}

// Store sends a batch of samples to the HTTP endpoint, the request is the proto marshaled
// and encoded bytes from codec.go.
func (c *Client) sendReq(ctx context.Context, req []byte) error {
	httpReq, err := http.NewRequest(http.MethodPost, c.urlString, bytes.NewReader(req))
	if err != nil {
		// Errors from NewRequest are from unparsable URLs, so are not
		// recoverable.
		return err
	}

	httpReq.Header.Add("Content-Encoding", "snappy")
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	httpResp, err := c.client.Do(httpReq.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("error sending request: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, httpResp.Body) //nolint:errcheck
		httpResp.Body.Close()
	}()

	if httpResp.StatusCode/100 != 2 {
		scanner := bufio.NewScanner(io.LimitReader(httpResp.Body, maxErrMsgLen))
		line := ""
		if scanner.Scan() {
			line = scanner.Text()
		}
		err = fmt.Errorf("server returned HTTP status %s: %s", httpResp.Status, line)
	}
	return err
}
