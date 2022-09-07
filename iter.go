package zip

import (
	"bufio"
	"fmt"
	"io"
)

var done = fmt.Errorf("no more ZIP file entries")

// HasNoMoreEntries checks if the given error indicates that
// there are no more constituent ZIP files that can be iterated.
func HasNoMoreEntries(err error) bool {
	return err == done
}

// Iterator allows for entries in a ZIP file to be accessed
// one by one using the Next method. This prevents high memory
// overheads while accessing ZIP files with several thousands
// of constituent files.
type Iterator struct {
	r         io.ReaderAt   // io handle for reading the underlying ZIP file
	Comment   string        // ZIP file comment, if any
	buf       *bufio.Reader // handle for section reader buffer
	size      int64
	dirOffset uint64
}

// NewIterator creates a new instance of a ZIP file iterator that
// accesses the constituent files from the given ZIP file reader.
func NewIterator(r io.ReaderAt, size int64) (*Iterator, error) {
	if size < 0 {
		return nil, fmt.Errorf("zip: size cannot be negative")
	}
	iter := new(Iterator)
	if err := iter.init(r, size); err != nil {
		return nil, err
	}
	return iter, nil
}

func (it *Iterator) init(r io.ReaderAt, size int64) error {
	end, err := readDirectoryEnd(r, size)
	if err != nil {
		return err
	}
	it.dirOffset = end.directoryOffset
	it.r = r
	it.size = size
	it.Comment = end.comment
	return it.Reset()
}

// Reset allows the iteration of the current ZIP file iterator
// to restart from the very first entry.
func (it *Iterator) Reset() error {
	rs := io.NewSectionReader(it.r, 0, it.size)
	if _, err := rs.Seek(int64(it.dirOffset), io.SeekStart); err != nil {
		return err
	}
	it.buf = bufio.NewReader(rs)
	return nil
}

// Next returns the consecutive file from the current ZIP file
// if it exists. Otherwise, it returns an error to indicate
// end of iteration or any other errors encountered.
func (it *Iterator) Next() (*File, error) {
	f := &File{zip: new(Reader), zipr: it.r}
	err := readDirectoryHeader(f, it.buf)
	if err == ErrFormat || err == io.ErrUnexpectedEOF {
		return nil, done
	}
	if err != nil {
		return nil, err
	}
	return f, nil
}

// Append appends entries to the existing zip archive represented by z.
// The writer w should be positioned at the end of the archive data.
// When the returned writer is closed, any entries with names that
// already exist in the archive will have been "replaced" by the new
// entries, although the original data will still be there.
func (it *Iterator) Append(w io.Writer) (*Writer, error) {
	return newAppendingWriter(it, w)
}

func newAppendingWriter(it *Iterator, fw io.Writer) (*Writer, error) {
	w := &Writer{
		cw: &countWriter{
			w:     bufio.NewWriter(fw),
			count: it.size,
		},
		names: make(map[string]int),
	}
	i := 0
	for f, err := it.Next(); !HasNoMoreEntries(err); f, err = it.Next() {
		if err != nil {
			return nil, err
		}
		w.dir = append(w.dir, &header{
			FileHeader: &f.FileHeader,
			offset:     uint64(f.headerOffset),
		})
		w.names[f.Name] = i
		i++
	}
	return w, nil
}
