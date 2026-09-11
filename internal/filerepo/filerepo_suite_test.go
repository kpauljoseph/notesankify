package filerepo_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFileRepo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "FileRepo Suite")
}
