//go:build integration

package pinecone

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Read capacity tests run on their own, outside RunSuites, so they can run against any control
// plane that serves the SDK's API version, including a local one: set PINECONE_CONTROLLER_HOST and
// PINECONE_API_KEY, then run
//
//	go test -count=1 -v -tags integration -run 'TestReadCapacity' ./pinecone
//
// Dedicated read capacity is billed and replica autoscaling is enabled per organization, so the
// tests run only when PINECONE_TEST_DRN_AUTOSCALING is set. They use the smallest shape: node type
// b1, 1 shard, 1-2 replicas.
//
// A change to dedicated read capacity takes effect asynchronously, and the API refuses the next
// change until it settles. By default the tests poll describe until the read capacity state is
// Ready. A control plane with no provisioner never gets there; for one, set
// PINECONE_TEST_RC_SETTLE_CMD to a command that marks an index's read capacity Ready. It's run
// with the index name as its last argument.

const (
	readCapacityTestNodeType = "b1"
	readCapacitySettleWait   = 10 * time.Minute
)

func newReadCapacityTestClient(t *testing.T) *Client {
	t.Helper()
	if os.Getenv("PINECONE_TEST_DRN_AUTOSCALING") == "" {
		t.Skip("set PINECONE_TEST_DRN_AUTOSCALING to run the dedicated read capacity tests (billed, organization-gated)")
	}
	apiKey := os.Getenv("PINECONE_API_KEY")
	require.NotEmpty(t, apiKey, "PINECONE_API_KEY env variable not set")
	pc, err := NewClient(NewClientParams{ApiKey: apiKey, SourceTag: "pinecone_test_go_sdk"})
	require.NoError(t, err)
	return pc
}

func readCapacityTestIndexName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func createReadCapacityTestIndex(t *testing.T, pc *Client, name string, readCapacity *ReadCapacityParams) *Index {
	t.Helper()
	ctx := context.Background()
	idx, err := pc.CreateServerlessIndex(ctx, &CreateServerlessIndexRequest{
		Name:         name,
		Dimension:    ptr(int32(setDimensionsForTestIndexes())),
		Metric:       ptr(IndexMetricCosine),
		Cloud:        CloudAWS,
		Region:       "us-east-1",
		ReadCapacity: readCapacity,
	})
	require.NoError(t, err)
	require.NotNil(t, idx)
	t.Cleanup(func() {
		if err := pc.DeleteIndex(context.Background(), name); err != nil {
			t.Logf("cleanup: deleting index %s: %v", name, err)
		}
	})
	return idx
}

func manualReadCapacity(replicas int32) *ReadCapacityParams {
	return &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
		NodeType: ptr(readCapacityTestNodeType),
		Shards:   ptr(int32(1)),
		Scaling:  &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(replicas)}},
	}}
}

func autoReadCapacity(minReplicas, maxReplicas int32) *ReadCapacityParams {
	return &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
		NodeType: ptr(readCapacityTestNodeType),
		Shards:   ptr(int32(1)),
		Scaling:  &ReadCapacityScaling{Auto: &ReadCapacityAutoScaling{MinReplicas: minReplicas, MaxReplicas: maxReplicas}},
	}}
}

func describeDedicated(t *testing.T, pc *Client, name string) *ReadCapacityDedicated {
	t.Helper()
	idx, err := pc.DescribeIndex(context.Background(), name)
	require.NoError(t, err)
	require.NotNil(t, idx.ReadCapacity, "ReadCapacity")
	require.NotNil(t, idx.ReadCapacity.Dedicated, "ReadCapacity.Dedicated")
	return idx.ReadCapacity.Dedicated
}

// settleReadCapacity waits until the index's read capacity can be changed again.
func settleReadCapacity(t *testing.T, pc *Client, name string) {
	t.Helper()
	if cmd := os.Getenv("PINECONE_TEST_RC_SETTLE_CMD"); cmd != "" {
		args := append(strings.Fields(cmd), name)
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "PINECONE_TEST_RC_SETTLE_CMD: %s", out)
		return
	}
	deadline := time.Now().Add(readCapacitySettleWait)
	for {
		idx, err := pc.DescribeIndex(context.Background(), name)
		require.NoError(t, err)
		settled, err := readCapacitySettled(idx)
		require.NoError(t, err, "read capacity of %s", name)
		if settled {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("read capacity of %s not Ready after %s", name, readCapacitySettleWait)
		}
		time.Sleep(10 * time.Second)
	}
}

func configureReadCapacity(t *testing.T, pc *Client, name string, rc *ReadCapacityParams) {
	t.Helper()
	_, err := pc.ConfigureIndex(context.Background(), name, ConfigureIndexParams{ReadCapacity: rc})
	require.NoError(t, err)
}

// requireAPIError checks that err is a *PineconeError with a 4xx status and, when errorCode isn't
// empty, that error code. It doesn't pin the exact status: the API may answer a malformed request
// with 400 or 422.
func requireAPIError(t *testing.T, err error, errorCode string) {
	t.Helper()
	require.Error(t, err)
	var pcErr *PineconeError
	require.True(t, errors.As(err, &pcErr), "want *PineconeError, got %T: %v", err, err)
	t.Logf("API error: %v", pcErr)
	assert.GreaterOrEqual(t, pcErr.Code, 400, "status")
	assert.Less(t, pcErr.Code, 500, "status")
	if errorCode != "" {
		assert.Equal(t, errorCode, pcErr.ErrorCode, "error code in %s", pcErr.Msg)
	}
}

func TestReadCapacityOnDemandIntegration(t *testing.T) {
	pc := newReadCapacityTestClient(t)
	name := readCapacityTestIndexName("rc-ondemand")
	createReadCapacityTestIndex(t, pc, name, nil)

	idx, err := pc.DescribeIndex(context.Background(), name)
	require.NoError(t, err)
	require.NotNil(t, idx.ReadCapacity)
	assert.NotNil(t, idx.ReadCapacity.OnDemand, "a new index defaults to OnDemand")
	assert.Nil(t, idx.ReadCapacity.Dedicated)

	list, err := pc.ListIndexes(context.Background())
	require.NoError(t, err)
	var found *Index
	for _, i := range list {
		if i.Name == name {
			found = i
		}
	}
	require.NotNil(t, found, "ListIndexes includes %s", name)
	require.NotNil(t, found.ReadCapacity)
	assert.NotNil(t, found.ReadCapacity.OnDemand)
}

// The v7.2.0 shape, with shards inside Scaling.Manual, still creates a Manual index, and describe
// reports shards in both the new and the deprecated field.
func TestReadCapacityManualV7ShapeIntegration(t *testing.T) {
	pc := newReadCapacityTestClient(t)
	name := readCapacityTestIndexName("rc-manual-v7")
	createReadCapacityTestIndex(t, pc, name, &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
		NodeType: ptr(readCapacityTestNodeType),
		Scaling:  &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(1)), Shards: ptr(int32(1))}},
	}})

	dedicated := describeDedicated(t, pc, name)
	assert.Equal(t, readCapacityTestNodeType, *dedicated.NodeType)
	assert.Equal(t, ptr(int32(1)), dedicated.Shards)
	assert.Equal(t, ptr(ReadCapacityScalingStrategyManual), dedicated.ScalingStrategy)
	require.NotNil(t, dedicated.Scaling)
	require.NotNil(t, dedicated.Scaling.Manual)
	assert.Nil(t, dedicated.Scaling.Auto)
	assert.Equal(t, ptr(int32(1)), dedicated.Scaling.Manual.Replicas)
	assert.Equal(t, ptr(int32(1)), dedicated.Scaling.Manual.Shards)
}

// Walks one index through Manual -> Auto -> new bounds -> shards only -> node type only -> Manual.
func TestReadCapacityTransitionsIntegration(t *testing.T) {
	pc := newReadCapacityTestClient(t)
	name := readCapacityTestIndexName("rc-transitions")
	createReadCapacityTestIndex(t, pc, name, manualReadCapacity(1))
	settleReadCapacity(t, pc, name)

	configureReadCapacity(t, pc, name, &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
		Scaling: &ReadCapacityScaling{Auto: &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 2}},
	}})
	dedicated := describeDedicated(t, pc, name)
	assert.Equal(t, ptr(ReadCapacityScalingStrategyAuto), dedicated.ScalingStrategy)
	require.NotNil(t, dedicated.Scaling)
	assert.Nil(t, dedicated.Scaling.Manual, "an autoscaled index has no fixed replica count")
	assert.Equal(t, &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 2}, dedicated.Scaling.Auto)
	settleReadCapacity(t, pc, name)

	configureReadCapacity(t, pc, name, &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
		Scaling: &ReadCapacityScaling{Auto: &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 3}},
	}})
	dedicated = describeDedicated(t, pc, name)
	assert.Equal(t, &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 3}, dedicated.Scaling.Auto)
	settleReadCapacity(t, pc, name)

	configureReadCapacity(t, pc, name, &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{Shards: ptr(int32(2))}})
	dedicated = describeDedicated(t, pc, name)
	assert.Equal(t, ptr(int32(2)), dedicated.Shards)
	assert.Equal(t, ptr(ReadCapacityScalingStrategyAuto), dedicated.ScalingStrategy, "a shards-only change keeps the strategy")
	assert.Equal(t, &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 3}, dedicated.Scaling.Auto)
	settleReadCapacity(t, pc, name)

	configureReadCapacity(t, pc, name, &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{NodeType: ptr("t1")}})
	dedicated = describeDedicated(t, pc, name)
	assert.Equal(t, "t1", *dedicated.NodeType)
	assert.Equal(t, ptr(ReadCapacityScalingStrategyAuto), dedicated.ScalingStrategy, "a node type change keeps the strategy")
	settleReadCapacity(t, pc, name)

	configureReadCapacity(t, pc, name, &ReadCapacityParams{Dedicated: &ReadCapacityDedicatedConfig{
		Scaling: &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(2))}},
	}})
	dedicated = describeDedicated(t, pc, name)
	assert.Equal(t, ptr(ReadCapacityScalingStrategyManual), dedicated.ScalingStrategy)
	require.NotNil(t, dedicated.Scaling.Manual)
	assert.Nil(t, dedicated.Scaling.Auto)
	assert.Equal(t, ptr(int32(2)), dedicated.Scaling.Manual.Replicas)
	assert.Equal(t, ptr(int32(2)), dedicated.Scaling.Manual.Shards)
}

func TestReadCapacityCreateAutoIntegration(t *testing.T) {
	pc := newReadCapacityTestClient(t)
	name := readCapacityTestIndexName("rc-auto")
	createReadCapacityTestIndex(t, pc, name, autoReadCapacity(1, 2))

	dedicated := describeDedicated(t, pc, name)
	assert.Equal(t, ptr(int32(1)), dedicated.Shards)
	assert.Equal(t, ptr(ReadCapacityScalingStrategyAuto), dedicated.ScalingStrategy)
	require.NotNil(t, dedicated.Scaling)
	assert.Nil(t, dedicated.Scaling.Manual)
	assert.Equal(t, &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 2}, dedicated.Scaling.Auto)
}

func TestReadCapacityOnDemandToDedicatedIntegration(t *testing.T) {
	pc := newReadCapacityTestClient(t)
	name := readCapacityTestIndexName("rc-od-to-auto")
	createReadCapacityTestIndex(t, pc, name, nil)

	configureReadCapacity(t, pc, name, autoReadCapacity(1, 2))
	dedicated := describeDedicated(t, pc, name)
	assert.Equal(t, readCapacityTestNodeType, *dedicated.NodeType)
	assert.Equal(t, ptr(int32(1)), dedicated.Shards)
	assert.Equal(t, &ReadCapacityAutoScaling{MinReplicas: 1, MaxReplicas: 2}, dedicated.Scaling.Auto)
}

// While a read capacity change is in progress, the next change is refused with ABORTED.
func TestReadCapacityConfigureWhileBusyIntegration(t *testing.T) {
	pc := newReadCapacityTestClient(t)
	name := readCapacityTestIndexName("rc-busy")
	createReadCapacityTestIndex(t, pc, name, manualReadCapacity(1))

	state := describeDedicated(t, pc, name).Status.State
	if state != "Migrating" && state != "Scaling" {
		t.Skipf("read capacity is already %q, so the index isn't busy", state)
	}
	_, err := pc.ConfigureIndex(context.Background(), name, ConfigureIndexParams{ReadCapacity: &ReadCapacityParams{
		Dedicated: &ReadCapacityDedicatedConfig{Scaling: &ReadCapacityScaling{Manual: &ReadCapacityManualScaling{Replicas: ptr(int32(2))}}},
	}})
	requireAPIError(t, err, "ABORTED")
}

// Run against an organization without replica autoscaling: set
// PINECONE_TEST_DRN_AUTOSCALING_DISABLED as well as PINECONE_TEST_DRN_AUTOSCALING.
func TestReadCapacityAutoNotEnabledIntegration(t *testing.T) {
	pc := newReadCapacityTestClient(t)
	if os.Getenv("PINECONE_TEST_DRN_AUTOSCALING_DISABLED") == "" {
		t.Skip("set PINECONE_TEST_DRN_AUTOSCALING_DISABLED for an organization without replica autoscaling")
	}
	name := readCapacityTestIndexName("rc-auto-off")
	_, err := pc.CreateServerlessIndex(context.Background(), &CreateServerlessIndexRequest{
		Name:         name,
		Dimension:    ptr(int32(setDimensionsForTestIndexes())),
		Metric:       ptr(IndexMetricCosine),
		Cloud:        CloudAWS,
		Region:       "us-east-1",
		ReadCapacity: autoReadCapacity(1, 2),
	})
	if err == nil {
		t.Cleanup(func() {
			if err := pc.DeleteIndex(context.Background(), name); err != nil {
				t.Logf("cleanup: deleting index %s: %v", name, err)
			}
		})
	}
	requireAPIError(t, err, "FORBIDDEN")
}
