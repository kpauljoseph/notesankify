package utils_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/pkg/utils"
)

var _ = Describe("BrowseStartDir", func() {
	var home string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
	})

	It("returns the current directory when it exists", func() {
		current := filepath.Join(home, "chosen")
		Expect(os.MkdirAll(current, 0o755)).To(Succeed())

		dir, err := utils.BrowseStartDir(current)

		Expect(err).NotTo(HaveOccurred())
		Expect(dir).To(Equal(current))
	})

	It("falls back to the app folder in the home directory when nothing is chosen", func() {
		dir, err := utils.BrowseStartDir("")

		Expect(err).NotTo(HaveOccurred())
		Expect(dir).To(Equal(filepath.Join(home, utils.DefaultBrowseDirName)))
		Expect(dir).To(BeADirectory())
	})

	It("falls back when the current path does not exist", func() {
		dir, err := utils.BrowseStartDir(filepath.Join(home, "missing"))

		Expect(err).NotTo(HaveOccurred())
		Expect(dir).To(Equal(filepath.Join(home, utils.DefaultBrowseDirName)))
	})

	It("falls back when the current path is a file", func() {
		file := filepath.Join(home, "notes.pdf")
		Expect(os.WriteFile(file, []byte("x"), 0o644)).To(Succeed())

		dir, err := utils.BrowseStartDir(file)

		Expect(err).NotTo(HaveOccurred())
		Expect(dir).To(Equal(filepath.Join(home, utils.DefaultBrowseDirName)))
	})

	It("never returns a macOS-protected folder as the fallback", func() {
		dir, err := utils.BrowseStartDir("")

		Expect(err).NotTo(HaveOccurred())
		for _, protected := range []string{"Desktop", "Documents", "Downloads"} {
			Expect(dir).NotTo(HavePrefix(filepath.Join(home, protected)))
		}
	})
})
