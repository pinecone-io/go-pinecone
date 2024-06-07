package useragent

import (
	"fmt"
	"strings"
)

func getPackageVersion() string {
	// update at release time
	return "v0.5.0"
}

func BuildUserAgent(sourceTag string) string {
	return buildUserAgent("go-client", sourceTag)
}

func BuildUserAgentGRPC(sourceTag string) string {
	return buildUserAgent("go-client[grpc]", sourceTag)
}

func buildUserAgent(appName string, sourceTag string) string {
	appVersion := getPackageVersion()

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
	var strBldr strings.Builder
	for _, char := range userAgent {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '_' || char == ' ' {
			strBldr.WriteRune(char)
		}
	}
	userAgent = strBldr.String()

	// Trim left/right whitespace
	userAgent = strings.TrimSpace(userAgent)

	// Condense multiple spaces to one, and replace with underscore
	userAgent = strings.Join(strings.Fields(userAgent), "_")

	return fmt.Sprintf("; source_tag=%s;", userAgent)
}
