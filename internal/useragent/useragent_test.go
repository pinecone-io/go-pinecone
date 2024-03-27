package useragent

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuildUserAgentNoSourceTag(t *testing.T) {
	sourceTag := ""
	expectedStartWith := fmt.Sprintf("go-client/%s", getPackageVersion())
	result := BuildUserAgent(sourceTag)
	if !strings.HasPrefix(result, expectedStartWith) {
		t.Errorf("BuildUserAgent(): expected user-agent to start with %s, but got %s", expectedStartWith, result)
	}
	if strings.Contains(result, "source_tag") {
		t.Errorf("BuildUserAgent(): expected user-agent to not contain 'source_tag', but got %s", result)
	}
}

func TestBuildUserAgentWithSourceTag(t *testing.T) {
	sourceTag := "my_source_tag"
	expectedStartWith := fmt.Sprintf("go-client/%s", getPackageVersion())
	result := BuildUserAgent(sourceTag)
	if !strings.HasPrefix(result, expectedStartWith) {
		t.Errorf("BuildUserAgent(): expected user-agent to start with %s, but got %s", expectedStartWith, result)
	}
	if !strings.Contains(result, "source_tag=my_source_tag") {
		t.Errorf("BuildUserAgent(): expected user-agent to contain 'source_tag=my_source_tag', but got %s", result)
	}
}

func TestBuildUserAgentGRPCNoSourceTag(t *testing.T) {
	sourceTag := ""
	expectedStartWith := fmt.Sprintf("go-client[grpc]/%s", getPackageVersion())
	result := BuildUserAgentGRPC(sourceTag)
	if !strings.HasPrefix(result, expectedStartWith) {
		t.Errorf("BuildUserAgent(): expected user-agent to start with %s, but got %s", expectedStartWith, result)
	}
	if strings.Contains(result, "source_tag") {
		t.Errorf("BuildUserAgent(): expected user-agent to not contain 'source_tag', but got %s", result)
	}
}

func TestBuildUserAgentGRPCWithSourceTag(t *testing.T) {
	sourceTag := "my_source_tag"
	expectedStartWith := fmt.Sprintf("go-client[grpc]/%s", getPackageVersion())
	result := BuildUserAgentGRPC(sourceTag)
	if !strings.HasPrefix(result, expectedStartWith) {
		t.Errorf("BuildUserAgent(): expected user-agent to start with %s, but got %s", expectedStartWith, result)
	}
	if !strings.Contains(result, "source_tag=my_source_tag") {
		t.Errorf("BuildUserAgent(): expected user-agent to contain 'source_tag=my_source_tag', but got %s", result)
	}
}

func TestBuildUserAgentSourceTagIsNormalized(t *testing.T) {
	sourceTag := "my source tag!!!!"
	result := BuildUserAgent(sourceTag)
	if !strings.Contains(result, "source_tag=my_source_tag") {
		t.Errorf("BuildUserAgent(\"%s\"): expected user-agent to contain 'source_tag=my_source_tag', but got %s", sourceTag, result)
	}

	sourceTag = "My Source Tag"
	result = BuildUserAgent(sourceTag)
	if !strings.Contains(result, "source_tag=my_source_tag") {
		t.Errorf("BuildUserAgent(\"%s\"): expected user-agent to contain 'source_tag=my_source_tag', but got %s", sourceTag, result)
	}

	sourceTag = "   My Source Tag  123  "
	result = BuildUserAgent(sourceTag)
	if !strings.Contains(result, "source_tag=my_source_tag") {
		t.Errorf("BuildUserAgent(\"%s\"): expected user-agent to contain 'source_tag=my_source_tag_123', but got %s", sourceTag, result)
	}

	sourceTag = "   My Source Tag  123 #### !! "
	result = BuildUserAgent(sourceTag)
	if !strings.Contains(result, "source_tag=my_source_tag") {
		t.Errorf("BuildUserAgent(\"%s\"): expected user-agent to contain 'source_tag=my_source_tag_123', but got %s", sourceTag, result)
	}
}
