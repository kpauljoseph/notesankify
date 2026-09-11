package filerepo_test

import (
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/storage/repository"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kpauljoseph/notesankify/internal/filerepo"
)

// stubRepo records which methods were called so the tests can prove that the
// quiet wrapper never reads a directory to decide whether it is listable.
type stubRepo struct {
	calls map[string]int
}

func newStub() *stubRepo { return &stubRepo{calls: map[string]int{}} }

func (s *stubRepo) hit(name string) { s.calls[name]++ }

func (s *stubRepo) Exists(fyne.URI) (bool, error)                { s.hit("Exists"); return true, nil }
func (s *stubRepo) Reader(fyne.URI) (fyne.URIReadCloser, error)  { s.hit("Reader"); return nil, nil }
func (s *stubRepo) CanRead(fyne.URI) (bool, error)               { s.hit("CanRead"); return true, nil }
func (s *stubRepo) Destroy(string)                               { s.hit("Destroy") }
func (s *stubRepo) Writer(fyne.URI) (fyne.URIWriteCloser, error) { s.hit("Writer"); return nil, nil }
func (s *stubRepo) CanWrite(fyne.URI) (bool, error)              { s.hit("CanWrite"); return true, nil }
func (s *stubRepo) Delete(fyne.URI) error                        { s.hit("Delete"); return nil }
func (s *stubRepo) Appender(fyne.URI) (fyne.URIWriteCloser, error) {
	s.hit("Appender")
	return nil, nil
}
func (s *stubRepo) CanList(fyne.URI) (bool, error)               { s.hit("CanList"); return true, nil }
func (s *stubRepo) List(fyne.URI) ([]fyne.URI, error)            { s.hit("List"); return nil, nil }
func (s *stubRepo) CreateListable(fyne.URI) error                { s.hit("CreateListable"); return nil }
func (s *stubRepo) Parent(u fyne.URI) (fyne.URI, error)          { s.hit("Parent"); return u, nil }
func (s *stubRepo) Child(u fyne.URI, _ string) (fyne.URI, error) { s.hit("Child"); return u, nil }
func (s *stubRepo) DeleteAll(fyne.URI) error                     { s.hit("DeleteAll"); return nil }
func (s *stubRepo) Copy(fyne.URI, fyne.URI) error                { s.hit("Copy"); return nil }
func (s *stubRepo) Move(fyne.URI, fyne.URI) error                { s.hit("Move"); return nil }

var _ = Describe("Quiet file repository", func() {
	var (
		stub *stubRepo
		repo repository.Repository
		dir  string
	)

	BeforeEach(func() {
		stub = newStub()
		repo = filerepo.Wrap(stub)
		dir = GinkgoT().TempDir()
	})

	Describe("CanList", func() {
		listable := func() repository.ListableRepository {
			l, ok := repo.(repository.ListableRepository)
			Expect(ok).To(BeTrue())
			return l
		}

		It("reports a directory as listable without reading it", func() {
			locked := filepath.Join(dir, "locked")
			Expect(os.Mkdir(locked, 0o755)).To(Succeed())
			// Execute-only: stat works, but opening or reading the directory fails.
			Expect(os.Chmod(locked, 0o100)).To(Succeed())
			DeferCleanup(os.Chmod, locked, 0o755)

			ok, err := listable().CanList(storage.NewFileURI(locked))

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(stub.calls).NotTo(HaveKey("CanList"))
			Expect(stub.calls).NotTo(HaveKey("List"))
		})

		It("reports a file as not listable", func() {
			file := filepath.Join(dir, "notes.pdf")
			Expect(os.WriteFile(file, []byte("x"), 0o644)).To(Succeed())

			ok, err := listable().CanList(storage.NewFileURI(file))

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})

		It("reports a missing path as not listable without an error", func() {
			ok, err := listable().CanList(storage.NewFileURI(filepath.Join(dir, "missing")))

			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	It("forwards every other operation to the wrapped repository", func() {
		u := storage.NewFileURI(dir)

		_, _ = repo.Exists(u)
		_, _ = repo.(repository.ListableRepository).List(u)
		_, _ = repo.(repository.HierarchicalRepository).Parent(u)
		_, _ = repo.(repository.HierarchicalRepository).Child(u, "x")
		_, _ = repo.(repository.WritableRepository).Writer(u)
		_ = repo.(repository.WritableRepository).Delete(u)
		_, _ = repo.(repository.AppendableRepository).Appender(u)
		_ = repo.(repository.DeleteAllRepository).DeleteAll(u)
		_ = repo.(repository.CopyableRepository).Copy(u, u)
		_ = repo.(repository.MovableRepository).Move(u, u)

		for _, name := range []string{"Exists", "List", "Parent", "Child", "Writer", "Delete", "Appender", "DeleteAll", "Copy", "Move"} {
			Expect(stub.calls).To(HaveKeyWithValue(name, 1), name)
		}
	})

	It("reports unsupported operations when the wrapped repository lacks them", func() {
		minimal := filerepo.Wrap(minimalRepo{})

		_, err := minimal.(repository.ListableRepository).List(storage.NewFileURI(dir))

		Expect(err).To(MatchError(repository.ErrOperationNotSupported))
	})

	Describe("Register", func() {
		It("replaces the file repository so dialog probes no longer read directories", func() {
			repository.Register("file", stub)

			Expect(filerepo.Register()).To(Succeed())

			lister, err := storage.ListerForURI(storage.NewFileURI(dir))
			Expect(err).NotTo(HaveOccurred())
			Expect(lister).NotTo(BeNil())
			Expect(stub.calls).NotTo(HaveKey("CanList"))

			_, _ = storage.List(lister)
			Expect(stub.calls).To(HaveKeyWithValue("List", 1))
		})
	})
})

// minimalRepo implements only the base Repository interface.
type minimalRepo struct{}

func (minimalRepo) Exists(fyne.URI) (bool, error)               { return false, nil }
func (minimalRepo) Reader(fyne.URI) (fyne.URIReadCloser, error) { return nil, nil }
func (minimalRepo) CanRead(fyne.URI) (bool, error)              { return false, nil }
func (minimalRepo) Destroy(string)                              {}
