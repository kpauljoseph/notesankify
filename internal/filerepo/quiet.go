// Package filerepo installs a file:// storage repository for Fyne whose
// listability check does not read directory contents.
//
// Fyne's folder dialog asks the repository whether every favourite and every
// entry it displays can be listed. The default implementation answers by
// opening the directory and reading an entry. On macOS that read is what
// triggers the privacy prompt for Desktop, Documents, Downloads, Photos,
// Music and network volumes, so the dialog produced a chain of prompts before
// it even appeared (issue #29). A stat call answers the same question without
// touching the contents, and macOS does not prompt for it.
package filerepo

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage/repository"
)

// Register wraps the repository currently registered for file:// URIs so that
// CanList only stats the path. Call it once after the Fyne app is created,
// which is when the default file repository is registered.
func Register() error {
	inner, err := repository.ForScheme("file")
	if err != nil {
		return err
	}
	repository.Register("file", Wrap(inner))
	return nil
}

// Wrap returns a repository that answers CanList with a stat call and forwards
// every other operation to inner. Operations inner does not support report
// repository.ErrOperationNotSupported.
func Wrap(inner repository.Repository) repository.Repository {
	return &quiet{inner: inner}
}

type quiet struct {
	inner repository.Repository
}

// CanList reports whether the URI is an existing directory. It never opens
// the directory, so it cannot trigger a macOS folder privacy prompt.
func (q *quiet) CanList(u fyne.URI) (bool, error) {
	info, err := os.Stat(u.Path())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return info.IsDir(), nil
}

// Repository

func (q *quiet) Exists(u fyne.URI) (bool, error)               { return q.inner.Exists(u) }
func (q *quiet) Reader(u fyne.URI) (fyne.URIReadCloser, error) { return q.inner.Reader(u) }
func (q *quiet) CanRead(u fyne.URI) (bool, error)              { return q.inner.CanRead(u) }
func (q *quiet) Destroy(scheme string)                         { q.inner.Destroy(scheme) }

// ListableRepository

func (q *quiet) List(u fyne.URI) ([]fyne.URI, error) {
	if r, ok := q.inner.(repository.ListableRepository); ok {
		return r.List(u)
	}
	return nil, repository.ErrOperationNotSupported
}

func (q *quiet) CreateListable(u fyne.URI) error {
	if r, ok := q.inner.(repository.ListableRepository); ok {
		return r.CreateListable(u)
	}
	return repository.ErrOperationNotSupported
}

// HierarchicalRepository

func (q *quiet) Parent(u fyne.URI) (fyne.URI, error) {
	if r, ok := q.inner.(repository.HierarchicalRepository); ok {
		return r.Parent(u)
	}
	return nil, repository.ErrOperationNotSupported
}

func (q *quiet) Child(u fyne.URI, component string) (fyne.URI, error) {
	if r, ok := q.inner.(repository.HierarchicalRepository); ok {
		return r.Child(u, component)
	}
	return nil, repository.ErrOperationNotSupported
}

// WritableRepository

func (q *quiet) Writer(u fyne.URI) (fyne.URIWriteCloser, error) {
	if r, ok := q.inner.(repository.WritableRepository); ok {
		return r.Writer(u)
	}
	return nil, repository.ErrOperationNotSupported
}

func (q *quiet) CanWrite(u fyne.URI) (bool, error) {
	if r, ok := q.inner.(repository.WritableRepository); ok {
		return r.CanWrite(u)
	}
	return false, repository.ErrOperationNotSupported
}

func (q *quiet) Delete(u fyne.URI) error {
	if r, ok := q.inner.(repository.WritableRepository); ok {
		return r.Delete(u)
	}
	return repository.ErrOperationNotSupported
}

// AppendableRepository

func (q *quiet) Appender(u fyne.URI) (fyne.URIWriteCloser, error) {
	if r, ok := q.inner.(repository.AppendableRepository); ok {
		return r.Appender(u)
	}
	return nil, repository.ErrOperationNotSupported
}

// DeleteAllRepository

func (q *quiet) DeleteAll(u fyne.URI) error {
	if r, ok := q.inner.(repository.DeleteAllRepository); ok {
		return r.DeleteAll(u)
	}
	return repository.ErrOperationNotSupported
}

// CopyableRepository

func (q *quiet) Copy(source, destination fyne.URI) error {
	if r, ok := q.inner.(repository.CopyableRepository); ok {
		return r.Copy(source, destination)
	}
	return repository.ErrOperationNotSupported
}

// MovableRepository

func (q *quiet) Move(source, destination fyne.URI) error {
	if r, ok := q.inner.(repository.MovableRepository); ok {
		return r.Move(source, destination)
	}
	return repository.ErrOperationNotSupported
}
