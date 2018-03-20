package main_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestVaultReadAws2env(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "VaultReadAws2env Suite")
}
