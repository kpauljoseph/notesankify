package pdf_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPDF(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "PDF Processing Unit Suite")
}
