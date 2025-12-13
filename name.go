package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	// maxKubernetesNameLength is the maximum length allowed for Kubernetes resource names.
	maxKubernetesNameLength = 63
	// hashLength is the length of the hash to apply to names.
	hashLength = 6
)

// GenerateName generates a valid Kubernetes name.
func GenerateName(name, suffix string) string {
	h := sha256.Sum256([]byte(name))
	hEncoded := hex.EncodeToString(h[:])[:hashLength]
	fullSuffix := hEncoded + "-" + suffix
	fullName := name + "-" + fullSuffix

	if len(fullName) <= maxKubernetesNameLength {
		return fullName
	}

	maxNameLength := maxKubernetesNameLength - len(fullSuffix) - 1 // -1 for the hyphen separator
	truncatedName := name[:maxNameLength]

	// Ensure the truncated name ends with a hyphen
	if !strings.HasSuffix(truncatedName, "-") {
		truncatedName += "-"
	}

	return truncatedName + fullSuffix
}
