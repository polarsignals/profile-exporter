// Copyright (c) 2022 The Parca Authors
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
//

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"time"

	prometheus "buf.build/gen/go/prometheus/prometheus/protocolbuffers/go"
	"github.com/alecthomas/kong"
	"github.com/apache/arrow/go/v14/arrow/array"
	"github.com/apache/arrow/go/v14/arrow/ipc"
	grun "github.com/oklog/run"
	queryv1alpha1 "github.com/parca-dev/parca/gen/proto/go/parca/query/v1alpha1"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/common/sigv4"
	"github.com/prometheus/prometheus/storage/remote/azuread"
	"golang.org/x/exp/slog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v2"
)

type flags struct {
	LogLevel   string `kong:"enum='error,warn,info,debug',help='Log level.',default='info'"`
	ConfigFile string `kong:"help='Path to the config file.',type='path',default='profile-exporter.yaml'"`
}

type ConfigFile struct {
	RemoteWrite *RemoteWriteConfig `yaml:"remote_write,omitempty"`
	ParcaConfig *ParcaConfig       `yaml:"parca,omitempty"`
	Queries     []QueryConfig      `yaml:"queries,omitempty"`
}

type RemoteWriteConfig struct {
	URL           *config.URL       `yaml:"url"`
	RemoteTimeout model.Duration    `yaml:"remote_timeout,omitempty"`
	Headers       map[string]string `yaml:"headers,omitempty"`

	// We cannot do proper Go type embedding below as the parser will then parse
	// values arbitrarily into the overflow maps of further-down types.
	HTTPClientConfig config.HTTPClientConfig `yaml:",inline"`
	SigV4Config      *sigv4.SigV4Config      `yaml:"sigv4,omitempty"`
	AzureADConfig    *azuread.AzureADConfig  `yaml:"azuread,omitempty"`
}

type ParcaConfig struct {
	Address            string `yaml:"address,omitempty"`
	BearerToken        string `yaml:"bearer_token,omitempty"`
	BearerTokenFile    string `yaml:"bearer_token_file,omitempty"`
	Insecure           bool   `yaml:"insecure"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type QueryConfig struct {
	Name     string            `yaml:"name,omitempty"`
	Query    string            `yaml:"query,omitempty"`
	Duration model.Duration    `yaml:"duration,omitempty"`
	Matchers []FunctionMatcher `yaml:"matchers,omitempty"`
}

type FunctionMatcher struct {
	Contains string `yaml:"contains,omitempty"`
}

func main() {
	flags := flags{}
	kong.Parse(&flags)
	if err := run(flags); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func logLevelFromString(level string) slog.Level {
	switch level {
	case "error":
		return slog.LevelError
	case "warn":
		return slog.LevelWarn
	case "info":
		return slog.LevelInfo
	case "debug":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

func run(flags flags) error {
	var g grun.Group
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: logLevelFromString(flags.LogLevel)}))

	content, err := os.ReadFile(flags.ConfigFile)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}

	cfg := ConfigFile{}
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return fmt.Errorf("unmarshal config file: %w", err)
	}

	if cfg.RemoteWrite == nil {
		return fmt.Errorf("remote_write config is required")
	}
	if cfg.RemoteWrite.RemoteTimeout == 0 {
		cfg.RemoteWrite.RemoteTimeout = model.Duration(30 * time.Second)
	}

	if cfg.ParcaConfig == nil {
		return fmt.Errorf("parca config is required")
	}

	if len(cfg.Queries) == 0 {
		return fmt.Errorf("at least one query is required")
	}

	conn, err := grpcConn(cfg.ParcaConfig)
	if err != nil {
		return fmt.Errorf("connect to parca: %w", err)
	}
	queryClient := queryv1alpha1.NewQueryServiceClient(conn)

	remoteWriteClient, err := NewClient(&ClientConfig{
		URL:              cfg.RemoteWrite.URL,
		Timeout:          cfg.RemoteWrite.RemoteTimeout,
		HTTPClientConfig: cfg.RemoteWrite.HTTPClientConfig,
		SigV4Config:      cfg.RemoteWrite.SigV4Config,
		AzureADConfig:    cfg.RemoteWrite.AzureADConfig,
		Headers:          cfg.RemoteWrite.Headers,
	})
	if err != nil {
		return fmt.Errorf("create remote write client: %w", err)
	}

	for _, q := range cfg.Queries {
		ctx, cancel := context.WithCancel(ctx)
		g.Add(func() error {
			return runCollectionLoop(
				ctx,
				logger,
				q,
				queryClient,
				remoteWriteClient,
			)
		}, func(error) {
			cancel()
		})
	}

	g.Add(grun.SignalHandler(ctx, os.Interrupt, os.Kill))
	return g.Run()
}

func runCollectionLoop(
	ctx context.Context,
	logger *slog.Logger,
	q QueryConfig,
	queryClient queryv1alpha1.QueryServiceClient,
	remoteWriteClient *Client,
) error {
	ticker := time.NewTicker(time.Duration(q.Duration))

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := runCollection(
				ctx,
				q,
				queryClient,
				remoteWriteClient,
			); err != nil {
				logger.Error("error during collection", "err", err)
			}
		}
	}
}

func runCollection(
	ctx context.Context,
	q QueryConfig,
	queryClient queryv1alpha1.QueryServiceClient,
	remoteWriteClient *Client,
) error {
	now := time.Now()
	resp, err := queryClient.Query(ctx, &queryv1alpha1.QueryRequest{
		Mode: queryv1alpha1.QueryRequest_MODE_MERGE,
		Options: &queryv1alpha1.QueryRequest_Merge{
			Merge: &queryv1alpha1.MergeProfile{
				Query: q.Query,
				Start: timestamppb.New(now.Add(-time.Duration(q.Duration))),
				End:   timestamppb.New(now),
			},
		},
		ReportType: queryv1alpha1.QueryRequest_REPORT_TYPE_TABLE_ARROW,
	})
	if err != nil {
		return fmt.Errorf("query profiling data: %w", err)
	}

	arrowTable := resp.GetTableArrow()
	if arrowTable == nil {
		return fmt.Errorf("no arrow table returned")
	}

	r, err := ipc.NewReader(bytes.NewReader(arrowTable.Record))
	if err != nil {
		return fmt.Errorf("create ipc reader: %w", err)
	}

	if !r.Next() {
		return fmt.Errorf("no records returned")
	}

	record := r.Record()
	schema := record.Schema()

	cumulativeFieldIndexes := schema.FieldIndices("cumulative")
	if len(cumulativeFieldIndexes) != 1 {
		return fmt.Errorf("cumulative field is not found")
	}
	cumulativeFieldIndex := cumulativeFieldIndexes[0]

	flatFieldIndexes := schema.FieldIndices("flat")
	if len(flatFieldIndexes) != 1 {
		return fmt.Errorf("flat field is not found")
	}
	flatFieldIndex := flatFieldIndexes[0]

	functionNameFieldIndexes := schema.FieldIndices("function_name")
	if len(functionNameFieldIndexes) != 1 {
		return fmt.Errorf("function_name field is not found")
	}
	functionNameFieldIndex := functionNameFieldIndexes[0]

	cumulative, ok := record.Column(cumulativeFieldIndex).(*array.Int64)
	if !ok {
		return fmt.Errorf("cumulative field is not int64")
	}

	flat, ok := record.Column(flatFieldIndex).(*array.Int64)
	if !ok {
		return fmt.Errorf("flat field is not int64")
	}

	functionName, ok := record.Column(functionNameFieldIndex).(*array.Dictionary)
	if !ok {
		return fmt.Errorf("function_name field is not dictionary")
	}

	functionNameDict, ok := functionName.Dictionary().(*array.String)
	if !ok {
		return fmt.Errorf("function_name dictionary is not string")
	}

	promTimestamp := now.UnixNano() / 1e6
	result := &prometheus.WriteRequest{
		Timeseries: []*prometheus.TimeSeries{{
			Labels: []*prometheus.Label{{
				Name:  "__name__",
				Value: "profile_exporter_root_cumulative_value",
			}, {
				Name:  "query",
				Value: q.Query,
			}, {
				Name:  "query_name",
				Value: q.Name,
			}},
			Samples: []*prometheus.Sample{{
				Timestamp: promTimestamp,
				Value:     float64(resp.Total),
			}},
		}},
	}

	for i := 0; i < int(record.NumRows()); i++ {
		functionName := functionNameDict.Value(functionName.GetValueIndex(i))
		for _, m := range q.Matchers {
			if strings.Contains(functionName, m.Contains) {
				result.Timeseries = append(result.Timeseries, &prometheus.TimeSeries{
					Labels: []*prometheus.Label{{
						Name:  "__name__",
						Value: "profile_exporter_flat_value",
					}, {
						Name:  "query",
						Value: q.Query,
					}, {
						Name:  "query_name",
						Value: q.Name,
					}, {
						Name:  "function_name",
						Value: functionName,
					}},
					Samples: []*prometheus.Sample{{
						Timestamp: promTimestamp,
						Value:     float64(flat.Value(i)),
					}},
				}, &prometheus.TimeSeries{
					Labels: []*prometheus.Label{{
						Name:  "__name__",
						Value: "profile_exporter_cumulative_value",
					}, {
						Name:  "query",
						Value: q.Query,
					}, {
						Name:  "query_name",
						Value: q.Name,
					}, {
						Name:  "function_name",
						Value: functionName,
					}},
					Samples: []*prometheus.Sample{{
						Timestamp: promTimestamp,
						Value:     float64(cumulative.Value(i)),
					}},
				})
			}
		}
	}

	if err := remoteWriteClient.Send(ctx, result); err != nil {
		return fmt.Errorf("write to remote write: %w", err)
	}

	return nil
}

func grpcConn(cfg *ParcaConfig) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{}
	if cfg.Insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		config := &tls.Config{
			//nolint:gosec
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(config)))
	}

	if cfg.BearerToken != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(&perRequestBearerToken{
			token:    cfg.BearerToken,
			insecure: cfg.Insecure,
		}))
	}

	if cfg.BearerTokenFile != "" {
		b, err := os.ReadFile(cfg.BearerTokenFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read bearer token from file: %w", err)
		}
		opts = append(opts, grpc.WithPerRPCCredentials(&perRequestBearerToken{
			token:    string(b),
			insecure: cfg.Insecure,
		}))
	}

	return grpc.Dial(cfg.Address, opts...)
}

type perRequestBearerToken struct {
	token    string
	insecure bool
}

func (t *perRequestBearerToken) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (t *perRequestBearerToken) RequireTransportSecurity() bool {
	return !t.insecure
}
