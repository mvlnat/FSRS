package handler

import (
	"errors"
	"net/url"
	"strings"
)

var errInvalidExternalLink = errors.New("invalid external link")

func normalizeOptionalExternalLink(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", errInvalidExternalLink
	}

	scheme := strings.ToLower(parsed.Scheme)
	if !parsed.IsAbs() || (scheme != "http" && scheme != "https") || parsed.Hostname() == "" {
		return "", errInvalidExternalLink
	}

	parsed.Scheme = scheme
	return parsed.String(), nil
}
