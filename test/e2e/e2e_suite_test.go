/*
Copyright 2024 Nikola Niksa.

Licensed under the MIT License.
See the LICENSE file in the project root for full license information.
*/

package e2e

import (
	"fmt"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Run e2e tests using the Ginkgo runner.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting dependency suite\n")
	RunSpecs(t, "e2e suite")
}
