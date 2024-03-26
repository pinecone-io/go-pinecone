package util

import (
	"fmt"
	"regexp"
	"strings"
)

func BuildUserAgent(sourceTag string) string {
	return buildUserAgent("go-client", sourceTag)
}

func BuildUserAgentGRPC(sourceTag string) string {
	return buildUserAgent("go-client[grpc]", sourceTag)
}

func buildUserAgent(appName string, sourceTag string) string {
	// need to set to actual current version
	appVersion := "0.0.1"

	sourceTagInfo := ""
	if sourceTag != "" {
		sourceTagInfo = buildSourceTagField(sourceTag)
	}
	userAgent := fmt.Sprintf("%s/%s%s", appName, appVersion, sourceTagInfo)
	return userAgent
}

func buildSourceTagField(userAgent string) string {
	// Lowercase
	userAgent = strings.ToLower(userAgent)

	// Limit charset to [a-z0-9_ ]
	re := regexp.MustCompile(`[^a-z0-9_ ]`)
	userAgent = re.ReplaceAllString(userAgent, "")

	// Trim left/right whitespace
	userAgent = strings.TrimSpace(userAgent)

	// Condense multiple spaces to one, and replace with underscore
	userAgent = strings.Join(strings.Fields(userAgent), "_")

	return fmt.Sprintf("; source_tag=%s;", userAgent)
}
