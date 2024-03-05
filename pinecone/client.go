package pinecone

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/deepmap/oapi-codegen/pkg/securityprovider"
	"github.com/pinecone-io/go-pinecone/internal/gen/control"
	"io"
	"net/http"
)

type Client struct {
	apiKey     string
	restClient *control.Client
}

func NewClient(apiKey string) (*Client, error) {
	apiKeyProvider, err := securityprovider.NewSecurityProviderApiKey("header", "Api-Key", apiKey)
	if err != nil {
		return nil, err
	}

	client, err := control.NewClient("https://api.pinecone.io", control.WithRequestEditorFn(apiKeyProvider.Intercept))
	if err != nil {
		return nil, err
	}

	c := Client{apiKey: apiKey, restClient: client}
	return &c, nil
}

func (c *Client) Index(host string) (*IndexConnection, error) {
	return c.IndexWithNamespace(host, "")
}

func (c *Client) IndexWithNamespace(host string, namespace string) (*IndexConnection, error) {
	idx, err := newIndexConnection(c.apiKey, host, namespace)
	if err != nil {
		return nil, err
	}
	return idx, nil
}

func (c *Client) ListIndexes(ctx context.Context) ([]*Index, error) {
	res, err := c.restClient.ListIndexes(ctx)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var indexList control.IndexList
	err = json.NewDecoder(res.Body).Decode(&indexList)
	if err != nil {
		return nil, err
	}

	indexes := make([]*Index, len(*indexList.Indexes))
	for i, idx := range *indexList.Indexes {
		indexes[i] = toIndex(&idx)
	}

	return indexes, nil
}

type CreatePodIndexRequest struct {
	Name             string
	Dimension        int32
	Metric           IndexMetric
	Environment      string
	PodType          string
	Shards           int32
	Replicas         int32
	SourceCollection *string
	MetadataConfig   *PodSpecMetadataConfig
}

func (req CreatePodIndexRequest) ReplicaCount() *int32 {
	x := minOne(req.Replicas)
	return &x
}

func (req CreatePodIndexRequest) ShardCount() *int32 {
	x := minOne(req.Shards)
	return &x
}

func (req CreatePodIndexRequest) TotalCount() *int {
	x := int(*req.ReplicaCount() * *req.ShardCount())
	return &x
}

func (c *Client) CreatePodIndex(ctx context.Context, in *CreatePodIndexRequest) (*Index, error) {
	metric := control.IndexMetric(in.Metric)
	req := control.CreateIndexRequest{
		Name:      in.Name,
		Dimension: in.Dimension,
		Metric:    &metric,
	}

	//add the spec to req.
	//because this is defined as an anon struct in the generated code, it must match exactly here.
	req.Spec = control.CreateIndexRequest_Spec{
		Pod: &struct {
			Environment    string `json:"environment"`
			MetadataConfig *struct {
				Indexed *[]string `json:"indexed,omitempty"`
			} `json:"metadata_config,omitempty"`
			PodType          control.PodSpecPodType   `json:"pod_type"`
			Pods             *int                     `json:"pods,omitempty"`
			Replicas         *control.PodSpecReplicas `json:"replicas,omitempty"`
			Shards           *control.PodSpecShards   `json:"shards,omitempty"`
			SourceCollection *string                  `json:"source_collection,omitempty"`
		}{
			Environment:      in.Environment,
			PodType:          in.PodType,
			Pods:             in.TotalCount(),
			Replicas:         in.ReplicaCount(),
			Shards:           in.ShardCount(),
			SourceCollection: in.SourceCollection,
		},
	}
	if in.MetadataConfig != nil {
		req.Spec.Pod.MetadataConfig = &struct {
			Indexed *[]string `json:"indexed,omitempty"`
		}{
			Indexed: in.MetadataConfig.Indexed,
		}
	}

	res, err := c.restClient.CreateIndex(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		var errResp control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errResp)
		if err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		return nil, fmt.Errorf("failed to create index: %s", errResp.Error.Message)
	}

	return decodeIndex(res.Body)
}

type CreateServerlessIndexRequest struct {
	Name      string
	Dimension int32
	Metric    IndexMetric
	Cloud     Cloud
	Region    string
}

func (c *Client) CreateServerlessIndex(ctx context.Context, in *CreateServerlessIndexRequest) (*Index, error) {
	metric := control.IndexMetric(in.Metric)
	req := control.CreateIndexRequest{
		Name:      in.Name,
		Dimension: in.Dimension,
		Metric:    &metric,
		Spec: control.CreateIndexRequest_Spec{
			Serverless: &control.ServerlessSpec{
				Cloud:  control.ServerlessSpecCloud(in.Cloud),
				Region: in.Region,
			},
		},
	}

	res, err := c.restClient.CreateIndex(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusCreated {
		var errResp control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errResp)
		if err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		return nil, fmt.Errorf("failed to create index: %s", errResp.Error.Message)
	}

	return decodeIndex(res.Body)
}

func (c *Client) DescribeIndex(ctx context.Context, idxName string) (*Index, error) {
	res, err := c.restClient.DescribeIndex(ctx, idxName)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var errResp control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errResp)
		if err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		return nil, fmt.Errorf("failed to describe idx: %s", errResp.Error.Message)
	}

	return decodeIndex(res.Body)
}

func (c *Client) DeleteIndex(ctx context.Context, idxName string) error {
	res, err := c.restClient.DeleteIndex(ctx, idxName)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		var errResp control.ErrorResponse
		err = json.NewDecoder(res.Body).Decode(&errResp)
		if err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}
		return fmt.Errorf("failed to delete index: %s", errResp.Error.Message)
	}

	return nil
}

func toIndex(idx *control.IndexModel) *Index {
	if idx == nil {
		return nil
	}

	spec := &IndexSpec{}
	if idx.Spec.Pod != nil {
		spec.Pod = &PodSpec{
			Environment:      idx.Spec.Pod.Environment,
			PodType:          idx.Spec.Pod.PodType,
			PodCount:         int32(idx.Spec.Pod.Pods),
			Replicas:         idx.Spec.Pod.Replicas,
			ShardCount:       idx.Spec.Pod.Shards,
			SourceCollection: idx.Spec.Pod.SourceCollection,
		}
		if idx.Spec.Pod.MetadataConfig != nil {
			spec.Pod.MetadataConfig = &PodSpecMetadataConfig{Indexed: idx.Spec.Pod.MetadataConfig.Indexed}
		}
	}
	if idx.Spec.Serverless != nil {
		spec.Serverless = &ServerlessSpec{
			Cloud:  Cloud(idx.Spec.Serverless.Cloud),
			Region: idx.Spec.Serverless.Region,
		}
	}
	status := &IndexStatus{
		Ready: idx.Status.Ready,
		State: IndexStatusState(idx.Status.State),
	}
	return &Index{
		Name:      idx.Name,
		Dimension: idx.Dimension,
		Host:      idx.Host,
		Metric:    IndexMetric(idx.Metric),
		Spec:      spec,
		Status:    status,
	}
}

func decodeIndex(resBody io.ReadCloser) (*Index, error) {
	var idx control.IndexModel
	err := json.NewDecoder(resBody).Decode(&idx)
	if err != nil {
		return nil, fmt.Errorf("failed to decode idx response: %w", err)
	}

	return toIndex(&idx), nil
}

func minOne(x int32) int32 {
	if x < 1 {
		return 1
	}
	return x
}
